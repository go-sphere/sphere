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
	//
	// Check the error first. A non-nil error means the attempt failed — the queue
	// is closed, the context was cancelled, the transport broke — and the bool
	// carries no meaning. Only when the error is nil does the bool report whether
	// a message was taken, with false meaning the queue was empty.
	//
	// A polling loop must therefore be written as:
	//
	//	v, ok, err := q.TryConsume(ctx, topic)
	//	if err != nil {
	//		return err
	//	}
	//	if !ok {
	//		time.Sleep(interval)
	//		continue
	//	}
	//
	// Branching on the bool first spins forever once the queue is closed, and
	// swallows every transport error on the way.
	TryConsume(ctx context.Context, topic string) (T, bool, error)

	// PurgeQueue removes all pending messages from the specified topic queue.
	PurgeQueue(ctx context.Context, topic string) error

	io.Closer
}

// PubSub provides publish-subscribe messaging capabilities with typed message support.
// Messages are broadcast to all active subscribers of a topic.
type PubSub[T any] interface {
	// Broadcast sends a message to all subscribers of the specified topic.
	//
	// Delivery is best-effort: implementations must not block on a subscriber
	// that cannot keep up, so a slow or stalled subscriber may miss messages.
	// A nil error means the message was handed to the transport, not that every
	// subscriber observed it. Use Queue when messages must not be dropped.
	Broadcast(ctx context.Context, topic string, data T) error

	// Subscribe registers a handler function to receive messages from the specified topic.
	// The handler will be called for each message received on the topic.
	//
	// The ctx governs only the setup of the subscription (the initial registration
	// call); it does not control the lifetime of the long-running subscription.
	// Cancelling ctx after Subscribe returns does NOT stop delivery. To stop
	// receiving messages, call UnsubscribeAll for the topic or Close the PubSub.
	Subscribe(ctx context.Context, topic string, handler func(data T) error) error

	// UnsubscribeAll removes all subscriptions for the specified topic and waits
	// for handlers already running on it to return.
	UnsubscribeAll(ctx context.Context, topic string) error

	// Close stops all subscriptions and waits for handlers already running to
	// return, so that once it returns no handler is still touching resources the
	// caller is about to release. Ordered shutdown depends on this: a task group
	// that closes the pubsub before the database or the log backend would
	// otherwise pull them out from under a handler still executing.
	io.Closer
}

// MessageQueue combines both queue and publish-subscribe messaging patterns.
// This interface provides maximum flexibility for messaging architectures.
type MessageQueue[T any] interface {
	Queue[T]
	PubSub[T]
}
