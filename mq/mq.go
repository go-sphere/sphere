package mq

import (
	"context"
	"io"
)

// Queue provides point-to-point messaging capabilities with typed message support.
// Messages are delivered to exactly one consumer, following FIFO ordering.
type Queue[T any] interface {
	// Publish sends a message to the specified topic queue.
	// The message will be delivered to one available consumer.
	Publish(ctx context.Context, topic string, data T) error

	// Consume retrieves the next available message from the specified topic queue.
	// This operation typically blocks until a message is available or the context is cancelled.
	Consume(ctx context.Context, topic string) (T, error)

	// TryConsume retrieves the next available message from the specified topic queue without blocking.
	// The returned bool indicates whether a message was found.
	// When bool is false, error should be nil.
	TryConsume(ctx context.Context, topic string) (T, bool, error)

	// PurgeQueue removes all pending messages from the specified topic queue.
	PurgeQueue(ctx context.Context, topic string) error

	io.Closer
}

// PubSub provides publish-subscribe messaging capabilities with typed message support.
// Messages are broadcast to all active subscribers of a topic.
type PubSub[T any] interface {
	// Broadcast sends a message to all subscribers of the specified topic.
	// All active subscribers will receive a copy of the message.
	Broadcast(ctx context.Context, topic string, data T) error

	// Subscribe registers a handler function to receive messages from the specified topic.
	// The handler will be called for each message received on the topic.
	Subscribe(ctx context.Context, topic string, handler func(data T) error) error

	// UnsubscribeAll removes all subscriptions for the specified topic.
	UnsubscribeAll(ctx context.Context, topic string) error

	io.Closer
}

// MessageQueue combines both queue and publish-subscribe messaging patterns.
// This interface provides maximum flexibility for messaging architectures.
type MessageQueue[T any] interface {
	Queue[T]
	PubSub[T]
}
