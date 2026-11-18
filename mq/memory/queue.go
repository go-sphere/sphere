package memory

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueClosed is returned after Close: Publish fails immediately;
// Consume and TryConsume return it once remaining messages are drained.
var ErrQueueClosed = errors.New("memory mq: queue is closed")

type topicQueue[T any] struct {
	ch    chan T
	freed chan struct{}
}

func newTopicQueue[T any](size int) *topicQueue[T] {
	return &topicQueue[T]{
		ch:    make(chan T, size),
		freed: make(chan struct{}, 1),
	}
}

func (tq *topicQueue[T]) signalFreed() {
	select {
	case tq.freed <- struct{}{}:
	default:
	}
}

// Queue implements an in-memory point-to-point message queue with typed message support.
// It provides FIFO message delivery to exactly one consumer per topic. Topics
// are created on first use. Create it with NewQueue; the zero value is not
// usable. Methods are safe for concurrent use.
type Queue[T any] struct {
	queueSize int
	queues    map[string]*topicQueue[T]

	mu     sync.RWMutex
	closed bool
	// done is closed exactly once by Close to signal shutdown. The per-topic
	// data channels are never closed and all sends run under mu.RLock, so a
	// Publish can neither send on a closed channel nor acknowledge a write after
	// Close has completed; blocked Publish/Consume calls observe shutdown
	// through done instead.
	done chan struct{}
}

// NewQueue creates a new memory-based queue with the specified options.
// The default queue size is 100 messages per topic.
func NewQueue[T any](opt ...Option) *Queue[T] {
	return newQueueWithOptions[T](newOptions(opt...))
}

func newQueueWithOptions[T any](opts *options) *Queue[T] {
	return &Queue[T]{
		queueSize: opts.queueSize,
		queues:    make(map[string]*topicQueue[T]),
		done:      make(chan struct{}),
	}
}

// Publish sends data to topic. A ctx that is already done returns ctx.Err()
// without enqueuing. Otherwise it blocks when the per-topic buffer is full
// until a consumer takes a message, ctx is done, or Close unblocks it with
// ErrQueueClosed.
func (q *Queue[T]) Publish(ctx context.Context, topic string, data T) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := q.getOrCreateQueue(topic); err != nil {
		return err
	}

	for {
		q.mu.RLock()
		if q.closed {
			q.mu.RUnlock()
			return ErrQueueClosed
		}
		var tq *topicQueue[T]
		if current, exists := q.queues[topic]; exists && current != nil {
			tq = current
		} else {
			q.mu.RUnlock()
			if _, err := q.getOrCreateQueue(topic); err != nil {
				return err
			}
			continue
		}
		select {
		case tq.ch <- data:
			if len(tq.ch) < cap(tq.ch) {
				// Wake any other waiting producer if buffer slots still remain.
				tq.signalFreed()
			}
			q.mu.RUnlock()
			return nil
		default:
		}
		q.mu.RUnlock()

		select {
		case <-tq.freed:
		case <-q.done:
			return ErrQueueClosed
		case <-ctx.Done():
			tq.signalFreed()
			return ctx.Err()
		}
	}
}

// Consume blocks until a message is available for topic, the queue is closed, or
// ctx is done. A ctx that is already done returns ctx.Err() without taking a
// message. Messages that were accepted before Close are still delivered:
// ErrQueueClosed is only reported once the topic has been drained, so shutting
// down never silently discards work the queue already acknowledged.
func (q *Queue[T]) Consume(ctx context.Context, topic string) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	tq, closed, err := q.consumeQueue(topic)
	if err != nil {
		return zero, err
	}
	if closed {
		return drain(tq)
	}

	select {
	case data := <-tq.ch:
		tq.signalFreed()
		return data, nil
	case <-q.done:
		// Close raced with this wait. A buffered message and q.done are both
		// ready now, and a plain select would pick between them at random, so
		// drain explicitly to keep the delivery guarantee deterministic.
		return drain(tq)
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// TryConsume returns the next buffered message without blocking. ctx is
// checked only at entry. An empty queue is (zero, false, nil). After Close,
// remaining messages are still returned; an empty closed queue is
// ErrQueueClosed. Check err first.
func (q *Queue[T]) TryConsume(ctx context.Context, topic string) (T, bool, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return zero, false, ctx.Err()
	default:
	}

	// Resolved read-only rather than through consumeQueue: TryConsume never
	// blocks, so it has no need for a channel to wait on, and polling an unknown
	// topic must not allocate a queue that nothing will ever publish to.
	q.mu.RLock()
	tq, exists := q.queues[topic]
	closed := q.closed
	q.mu.RUnlock()

	if exists && tq != nil {
		select {
		case data := <-tq.ch:
			tq.signalFreed()
			return data, true, nil
		default:
		}
	}
	if closed {
		return zero, false, ErrQueueClosed
	}
	return zero, false, nil
}

// drain returns a message that is already buffered, or ErrQueueClosed when the
// topic is empty.
func drain[T any](tq *topicQueue[T]) (T, error) {
	var zero T
	select {
	case data := <-tq.ch:
		tq.signalFreed()
		return data, nil
	default:
		return zero, ErrQueueClosed
	}
}

// PurgeQueue drains every buffered message for topic. A missing topic is a
// no-op. After Close it returns ErrQueueClosed without draining. ctx can
// cancel a drain in progress.
func (q *Queue[T]) PurgeQueue(ctx context.Context, topic string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.RLock()
	tq, exists := q.queues[topic]
	closed := q.closed
	q.mu.RUnlock()

	if closed {
		return ErrQueueClosed
	}
	if !exists || tq == nil {
		return nil
	}

	// Drain at most what the buffer can hold: a producer publishing as fast as
	// this loop discards would otherwise keep the call alive indefinitely. The
	// topic channels are never closed, so there is no closed-channel case.
	for range cap(tq.ch) {
		select {
		case <-tq.ch:
			tq.signalFreed()
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	return nil
}

// DeleteQueue removes the topic's queue entry so its memory can be reclaimed.
// getOrCreateQueue only ever adds entries, so long-lived queues accumulate one
// channel per topic that is never released; DeleteQueue is the explicit cleanup
// hook for topics that are no longer in use.
//
// Any messages still buffered for the topic are discarded. Callers must only
// delete a topic that is idle (no active publishers or consumers): a Consume
// blocked on the old channel will not observe messages published after the
// entry is recreated by a subsequent Publish. A subsequent Publish/Consume for
// the same topic transparently recreates the queue.
func (q *Queue[T]) DeleteQueue(ctx context.Context, topic string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}
	if tq, exists := q.queues[topic]; exists && tq != nil {
		delete(q.queues, topic)
		tq.signalFreed()
	}
	return nil
}

// Close stops the queue. Buffered messages remain until Consume or
// TryConsume drain them; later Publish returns ErrQueueClosed. Close is
// idempotent.
func (q *Queue[T]) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	// Signal shutdown via done rather than closing the data channels, so a
	// Publish that is concurrently sending never panics on a closed channel.
	close(q.done)
	return nil
}

func (q *Queue[T]) getOrCreateQueue(topic string) (*topicQueue[T], error) {
	q.mu.RLock()
	tq, exists := q.queues[topic]
	closed := q.closed
	q.mu.RUnlock()
	if closed {
		return nil, ErrQueueClosed
	}
	if exists && tq != nil {
		return tq, nil
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, ErrQueueClosed
	}
	tq, exists = q.queues[topic]
	if !exists || tq == nil {
		tq = newTopicQueue[T](q.queueSize)
		q.queues[topic] = tq
	}
	return tq, nil
}

// consumeQueue resolves the channel a blocked Consume waits on. Unlike
// getOrCreateQueue it still resolves after Close (reporting closed=true) so a
// consumer can drain messages accepted before the shutdown; it never creates a
// queue for an unseen topic once closed.
func (q *Queue[T]) consumeQueue(topic string) (*topicQueue[T], bool, error) {
	q.mu.RLock()
	tq, exists := q.queues[topic]
	closed := q.closed
	q.mu.RUnlock()
	if exists && tq != nil {
		return tq, closed, nil
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	tq, exists = q.queues[topic]
	if q.closed {
		if !exists || tq == nil {
			return nil, true, ErrQueueClosed
		}
		return tq, true, nil
	}
	if !exists || tq == nil {
		tq = newTopicQueue[T](q.queueSize)
		q.queues[topic] = tq
	}
	return tq, false, nil
}
