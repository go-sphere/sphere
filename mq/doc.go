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
// Publish appends one record to the topic, and Consume or TryConsume removes
// it, so each record goes to exactly one consumer. FIFO is the order in which
// records leave the queue; with several consumers, the order in which they
// finish processing is up to the application. memory Publish blocks when the
// per-topic buffer is full; redis RPUSH is unbounded. TryConsume: check the
// error first — a non-nil error means the message was not delivered, and the
// bool must not be read as "nothing was waiting".
//
// Removal is destructive and there is no acknowledgement. A record is gone
// once a consumer has taken it: a consumer that crashes or fails while
// processing it loses it, and so does a redis consumer that cannot decode it
// (redis.DecodeError carries the raw bytes). Nothing is redelivered. A Publish
// retried after an error can enqueue the record twice. Background jobs that
// need retries belong in scheduler/asynq (scheduler.WithMaxRetry), with
// idempotent handlers.
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
