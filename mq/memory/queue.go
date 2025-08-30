package memory

import (
	"context"
	"fmt"
	"sync"
)

// Queue implements an in-memory point-to-point message queue with typed message support.
// It provides FIFO message delivery to exactly one consumer per topic.
type Queue[T any] struct {
	queueSize int
	queues    map[string]chan T

	mu     sync.RWMutex
	closed bool
}

// NewQueue creates a new memory-based queue with the specified options.
// The default queue size is 100 messages per topic.
func NewQueue[T any](opt ...Option) *Queue[T] {
	opts := newOptions(opt...)
	return &Queue[T]{
		queueSize: opts.queueSize,
		queues:    make(map[string]chan T),
	}
}

func (q *Queue[T]) Publish(ctx context.Context, topic string, data T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed")
	}
	if _, exists := q.queues[topic]; !exists {
		q.queues[topic] = make(chan T, q.queueSize)
	}

	select {
	case q.queues[topic] <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Queue[T]) Consume(ctx context.Context, topic string) (T, error) {
	q.mu.RLock()
	queue, exists := q.queues[topic]
	closed := q.closed
	q.mu.RUnlock()

	var zero T
	if closed {
		return zero, fmt.Errorf("queue is closed")
	}
	if !exists {
		return zero, fmt.Errorf("queue %s does not exist", topic)
	}

	select {
	case data := <-queue:
		return data, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (q *Queue[T]) PurgeQueue(ctx context.Context, topic string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed")
	}
	if queue, exists := q.queues[topic]; exists {
		for {
			select {
			case <-queue:
			default:
				return nil
			}
		}
	}

	return nil
}

func (q *Queue[T]) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	for _, ch := range q.queues {
		close(ch)
	}
	return nil
}
