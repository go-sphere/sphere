// Package mq is the typed messaging contract: point-to-point [Queue] and
// best-effort [PubSub], plus [MessageQueue] that combines both.
//
// Drivers live in subpackages: mq/memory (process-local channels, default
// buffer 100) and mq/redis (Redis lists and pub/sub, JSON by default, over a
// caller-owned *redis.Client).
//
// # Usage
//
//	import (
//		"context"
//
//		"github.com/go-sphere/sphere/mq/memory"
//	)
//
//	q := memory.NewMessageQueue[string]()
//	if err := q.Publish(ctx, "jobs", "resize"); err != nil {
//		return err
//	}
//	job, err := q.Consume(ctx, "jobs")
//
//	sub, err := q.Subscribe(ctx, "events", func(ctx context.Context, v string) error {
//		return nil
//	})
//	err = q.Broadcast(ctx, "events", "ready")
//
//	// Cleanup: Stop closes the queue and waits for PubSub handlers.
//	err = q.Stop(ctx)
//
// # Queue
//
// Publish delivers to exactly one consumer, FIFO. memory Publish blocks when
// the per-topic buffer is full; redis RPUSH is unbounded. TryConsume: check
// the error first — a non-nil error means the message was not delivered, and
// the bool must not be read as "nothing was waiting".
//
// Close is driver-split. memory Close stops the queue; Consume and TryConsume
// drain remaining messages and then return an error. redis Close is a no-op:
// the caller owns the *redis.Client.
//
// # PubSub
//
// Broadcast is best-effort: a slow subscriber may miss messages. Subscribe's
// ctx owns the returned [Subscription]: cancellation stops it and is
// propagated to its [Handler]. PubSub shutdown is deliberately two-phase.
// RequestStop and StopTopic only request cancellation and are safe to call
// from inside a Handler; Done closes after every Handler has returned. PubSub
// implements task.Task, whose Stop performs a context-bounded wait for that
// quiescence. After shutdown, Broadcast and Subscribe return
// [ErrPubSubClosed].
//
// Share a Redis client with cache only if you never call cache.DelAll
// (FlushDB) on that database.
package mq
