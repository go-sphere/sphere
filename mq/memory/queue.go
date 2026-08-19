package memory

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrNoMessage   = errors.New("memory mq: no message available")
	ErrQueueClosed = errors.New("memory mq: queue is closed")
)

// Queue implements an in-memory point-to-point message queue with typed message support.
// It provides FIFO message delivery to exactly one consumer per topic.
type Queue[T any] struct {
	queueSize int
	queues    map[string]chan T

	mu     sync.RWMutex
	closed bool
	// done is closed exactly once by Close to signal shutdown. The per-topic
	// data channels are never closed, so a concurrent Publish can never send
	// on a closed channel; blocked Publish/Consume calls observe shutdown
	// through done instead.
	done chan struct{}
}

// NewQueue creates a new memory-based queue with the specified options.
// The default queue size is 100 messages per topic.
func NewQueue[T any](opt ...Option) *Queue[T] {
	opts := newOptions(opt...)
	return &Queue[T]{
		queueSize: opts.queueSize,
		queues:    make(map[string]chan T),
		done:      make(chan struct{}),
	}
}

func (q *Queue[T]) Publish(ctx context.Context, topic string, data T) error {
	queue, err := q.getOrCreateQueue(topic)
	if err != nil {
		return err
	}

	select {
	case queue <- data:
		return nil
	case <-q.done:
		return ErrQueueClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Consume blocks until a message is available for topic, the queue is closed, or
// ctx is done. Messages that were accepted before Close are still delivered:
// ErrQueueClosed is only reported once the topic has been drained, so shutting
// down never silently discards work the queue already acknowledged.
func (q *Queue[T]) Consume(ctx context.Context, topic string) (T, error) {
	queue, closed, err := q.consumeQueue(topic)
	var zero T
	if err != nil {
		return zero, err
	}
	if closed {
		return drain(queue)
	}

	select {
	case data := <-queue:
		return data, nil
	case <-q.done:
		// Close raced with this wait. A buffered message and q.done are both
		// ready now, and a plain select would pick between them at random, so
		// drain explicitly to keep the delivery guarantee deterministic.
		return drain(queue)
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (q *Queue[T]) TryConsume(ctx context.Context, topic string) (T, bool, error) {
	var zero T
	select {
	case <-ctx.Done():
		return zero, false, ctx.Err()
	default:
	}

	// Resolved read-only rather than through consumeQueue: TryConsume never
	// blocks, so it has no need for a channel to wait on, and polling an unknown
	// topic must not allocate a queue that nothing will ever publish to.
	q.mu.RLock()
	queue, exists := q.queues[topic]
	closed := q.closed
	q.mu.RUnlock()

	if exists {
		select {
		case data := <-queue:
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
func drain[T any](queue chan T) (T, error) {
	var zero T
	select {
	case data := <-queue:
		return data, nil
	default:
		return zero, ErrQueueClosed
	}
}

func (q *Queue[T]) PurgeQueue(ctx context.Context, topic string) error {
	q.mu.RLock()
	queue, exists := q.queues[topic]
	closed := q.closed
	q.mu.RUnlock()

	if closed {
		return ErrQueueClosed
	}
	if !exists {
		return nil
	}

	for {
		select {
		case _, ok := <-queue:
			if !ok {
				return ErrQueueClosed
			}
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
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
	if err := ctx.Err(); err != nil {
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}
	delete(q.queues, topic)
	return nil
}

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

func (q *Queue[T]) getOrCreateQueue(topic string) (chan T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, ErrQueueClosed
	}
	queue, exists := q.queues[topic]
	if !exists {
		queue = make(chan T, q.queueSize)
		q.queues[topic] = queue
	}
	return queue, nil
}

// consumeQueue resolves the channel a blocked Consume waits on. Unlike
// getOrCreateQueue it still resolves after Close (reporting closed=true) so a
// consumer can drain messages accepted before the shutdown; it never creates a
// queue for an unseen topic once closed.
func (q *Queue[T]) consumeQueue(topic string) (chan T, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	queue, exists := q.queues[topic]
	if q.closed {
		if !exists {
			return nil, true, ErrQueueClosed
		}
		return queue, true, nil
	}
	if !exists {
		queue = make(chan T, q.queueSize)
		q.queues[topic] = queue
	}
	return queue, false, nil
}
