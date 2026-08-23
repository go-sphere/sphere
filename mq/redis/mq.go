// Package redis is the Redis-backed mq.MessageQueue: lists for Queue
// (RPUSH/BLPOP/LPOP) and Redis pub/sub for PubSub.
//
// WithClient is required and the client is never closed here. Default codec
// is JSON. Queue Close is a no-op. Consume cancellation is observed within
// about 1s (BLPOP poll). Decode failures after pop return DecodeError with
// the raw bytes; TryConsume still reports found=true — check err first.
// After PubSub Close, Subscribe returns ErrPubSubClosed; Broadcast still
// PUBLISHes. Topic names are Redis keys: they collide with cache keys on
// the same DB.
package redis

import "errors"

// MessageQueue embeds the redis Queue and PubSub. Close joins both halves
// (Queue Close is a no-op; PubSub Close stops subscriptions). The Redis
// client is not closed.
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

// Close closes the Queue (a no-op) and PubSub halves and joins their errors.
func (p *MessageQueue[T]) Close() error {
	return errors.Join(
		p.Queue.Close(),
		p.PubSub.Close(),
	)
}
