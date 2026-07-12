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

func (q *Queue[T]) Consume(ctx context.Context, topic string) (T, error) {
	queue, err := q.getOrCreateQueue(topic)
	var zero T
	if err != nil {
		return zero, err
	}

	select {
	case data, ok := <-queue:
		if !ok {
			return zero, ErrQueueClosed
		}
		return data, nil
	case <-q.done:
		return zero, ErrQueueClosed
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (q *Queue[T]) TryConsume(ctx context.Context, topic string) (T, bool, error) {
	queue, exists, err := q.getQueue(topic)
	var zero T
	if err != nil {
		return zero, false, err
	}
	if !exists {
		return zero, false, nil
	}
	select {
	case <-ctx.Done():
		return zero, false, ctx.Err()
	default:
	}

	select {
	case data, ok := <-queue:
		if !ok {
			return zero, false, ErrQueueClosed
		}
		return data, true, nil
	default:
		return zero, false, nil
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

func (q *Queue[T]) getQueue(topic string) (chan T, bool, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return nil, false, ErrQueueClosed
	}
	queue, exists := q.queues[topic]
	return queue, exists, nil
}
