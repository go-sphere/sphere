// Package mq is the typed messaging contract: point-to-point Queue and
// best-effort PubSub, plus MessageQueue that combines both.
//
// Drivers live in subpackages: memory (process-local channels, default buffer
// 100) and redis (lists + Redis pub/sub, JSON by default).
//
// # Queue
//
// Publish delivers to exactly one consumer, FIFO. memory Publish blocks when
// the per-topic buffer is full; redis RPUSH is unbounded. TryConsume: check
// the error first — a non-nil error means the bool carries no meaning.
//
// Close is driver-split. memory Close stops the queue and drains remaining
// messages, then Consume/TryConsume return ErrQueueClosed. redis Close is a
// no-op: the caller owns the *redis.Client.
//
// # PubSub
//
// Broadcast is best-effort: a slow subscriber may miss messages. Subscribe's
// ctx owns the returned Subscription: cancellation stops it and is propagated
// to its Handler. PubSub shutdown is deliberately two-phase. RequestStop and
// StopTopic only request cancellation and are safe to call from inside a
// Handler; Done closes after every Handler has returned. PubSub implements
// task.Task, whose Stop performs a context-bounded wait for that quiescence.
//
// Share a Redis client with cache only if you never call cache.DelAll
// (FlushDB) on that database.
package mq

import (
	"context"
	"errors"
	"io"

	"github.com/go-sphere/sphere/core/task"
)

// ErrPubSubClosed is returned by Broadcast and Subscribe after
// PubSub.RequestStop or task.Task.Stop.
var ErrPubSubClosed = errors.New("mq: pubsub closed")

// Queue is point-to-point messaging: one consumer, FIFO. Close semantics are
// driver-split — memory stops the queue; redis Close is a no-op.
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

// Handler processes one PubSub message. ctx is the lifetime context of the
// Subscription and is cancelled before the subscription begins stopping.
// Handlers should use it to interrupt blocking work promptly.
type Handler[T any] func(ctx context.Context, data T) error

// Subscription is the lifecycle handle returned by PubSub.Subscribe.
//
// Stop requests cancellation and returns without waiting for the Handler, so
// it is safe to call from inside that Handler. Done closes once the consumer
// goroutine and any running Handler have returned. Stop is idempotent.
type Subscription interface {
	Stop() error
	Done() <-chan struct{}
}

// PubSub provides best-effort typed publish-subscribe delivery.
//
// Lifecycle is two-phase so shutdown is reentrant-safe: RequestStop and
// StopTopic request cancellation; Done (or the channel returned by StopTopic)
// reports quiescence. The embedded task.Task makes a PubSub directly usable in
// task.Group and boot.Run. Start is the blocking lifecycle wait; it does not
// gate Subscribe or Broadcast, so subscriptions may be registered first.
type PubSub[T any] interface {
	task.Task

	// Broadcast sends a message to all subscribers of the specified topic.
	//
	// Delivery is best-effort: implementations must not block on a subscriber
	// that cannot keep up, so a slow or stalled subscriber may miss messages.
	// A nil error means the message was handed to the transport, not that every
	// subscriber observed it. Use Queue when messages must not be dropped.
	Broadcast(ctx context.Context, topic string, data T) error

	// Subscribe registers handler on topic. ctx controls the full subscription
	// lifetime, not just setup; cancelling it stops the subscription. The caller
	// owns the returned handle and may call Stop independently of the PubSub.
	Subscribe(ctx context.Context, topic string, handler Handler[T]) (Subscription, error)

	// StopTopic requests cancellation of every subscription currently registered
	// on topic. It does not wait and is safe to call from a Handler. The returned
	// channel closes when those subscriptions are quiescent. Later Subscribe calls
	// create a new generation and are not covered by that channel.
	StopTopic(topic string) (<-chan struct{}, error)

	// RequestStop requests cancellation of every subscription and prevents future
	// Broadcast and Subscribe calls. It does not wait for Handlers and is safe to
	// call from a Handler. RequestStop is idempotent.
	RequestStop() error

	// Done closes after RequestStop has been called and every subscription and
	// running Handler has returned.
	Done() <-chan struct{}
}

// MessageQueue is Queue plus PubSub. Its Close method closes the Queue and
// requests PubSub stop without waiting; task.Task.Stop closes the Queue and
// waits for PubSub quiescence.
type MessageQueue[T any] interface {
	Queue[T]
	PubSub[T]
}
