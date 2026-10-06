// Package memory is the process-local mq.MessageQueue: buffered channels for
// Queue and PubSub. Use it in tests, single-process deployments, or as the
// default driver before Redis is needed.
//
// Entry points: [NewQueue], [NewPubSub], or [NewMessageQueue] for both, each
// configured with [WithQueueSize] and [WithIdentifier]. None of them need
// external resources.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/mq/memory"
//
//	q := memory.NewQueue[string](memory.WithQueueSize(16))
//	defer q.Close()
//	if err := q.Publish(ctx, "jobs", "resize"); err != nil {
//		return err
//	}
//	job, err := q.Consume(ctx, "jobs")
//
// # Behavior
//
// The default buffer is 100 per topic (Queue) and per subscription (PubSub).
// Queue Publish blocks when the buffer is full; PubSub Broadcast drops and
// logs. Queue Close is idempotent and returns nil; Consume and TryConsume
// drain remaining buffered messages and then return [ErrQueueClosed].
// Subscribe's context owns the subscription. PubSub RequestStop is
// non-blocking; Stop waits for handler quiescence. [MessageQueue] Close stops
// both halves (errors.Join). PubSub and MessageQueue implement task.Task; use
// WithIdentifier when a group contains more than one. [Queue.DeleteQueue] is
// extra API, not part of mq.Queue.
package memory
