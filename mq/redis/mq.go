package redis

import "errors"

// MessageQueue combines both queue and publish-subscribe functionality in a single Redis-based implementation.
// It provides both point-to-point messaging and broadcast messaging capabilities using Redis as the backend.
type MessageQueue[T any] struct {
	*Queue[T]
	*PubSub[T]
}

// NewMessageQueue creates a new Redis-based message queue that supports both queue and pub/sub operations.
// Both components share the same Redis client and codec configuration.
func NewMessageQueue[T any](opt ...Option) (*MessageQueue[T], error) {
	queue, err := NewQueue[T](opt...)
	if err != nil {
		return nil, err
	}
	pubSub, err := NewPubSub[T](opt...)
	if err != nil {
		return nil, err
	}
	return &MessageQueue[T]{
		Queue:  queue,
		PubSub: pubSub,
	}, nil
}

func (p *MessageQueue[T]) Close() error {
	return errors.Join(
		p.Queue.Close(),
		p.PubSub.Close(),
	)
}
