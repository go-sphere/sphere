// Package redis is the Redis-backed mq.MessageQueue: lists for Queue
// (RPUSH/BLPOP/LPOP) and Redis pub/sub for PubSub.
//
// WithClient is required and the client is never closed here. Default codec
// is JSON. Queue Close is a no-op. Consume cancellation is observed within
// about 1s (BLPOP poll). Decode failures after pop return DecodeError with
// the raw bytes; TryConsume still reports found=true — check err first.
// PubSub RequestStop cancels subscriptions but never closes the injected client;
// Broadcast and Subscribe then return mq.ErrPubSubClosed. Topic names are
// Redis keys: they collide with cache keys on the same DB. PubSub and
// MessageQueue implement task.Task; use WithIdentifier when a group contains
// more than one.
package redis

import (
	"context"
	"errors"
)

// MessageQueue embeds the Redis Queue and PubSub. Close joins Queue.Close (a
// no-op) with PubSub.RequestStop; task.Task.Stop waits for PubSub quiescence.
// The injected Redis client is not closed.
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

// Close closes the Queue (a no-op) and requests PubSub stop. It does not wait
// for handlers; task lifecycle owners should call Stop.
func (p *MessageQueue[T]) Close() error {
	return errors.Join(
		p.Queue.Close(),
		p.RequestStop(),
	)
}

// Stop implements task.Task by closing the Queue (a no-op) and waiting for
// PubSub handlers to return or ctx to expire. The injected Redis client remains
// open.
func (p *MessageQueue[T]) Stop(ctx context.Context) error {
	return errors.Join(
		p.Queue.Close(),
		p.PubSub.Stop(ctx),
	)
}
