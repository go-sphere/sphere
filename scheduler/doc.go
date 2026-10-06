// Package scheduler is the contract for periodic jobs and asynchronous task
// queues. Drivers expose only the capabilities they support:
//
//   - [Cron]: Register/Unregister periodic jobs by name and cron spec.
//   - [Producer]: Enqueue named payloads with [EnqueueOption] values.
//   - [Consumer]: Handle payloads by exact kind.
//   - [Scheduler]: Cron + Producer + Consumer + io.Closer.
//
// Drivers: package scheduler/cron wraps robfig/cron (Cron only) and package
// scheduler/asynq wraps hibiken/asynq over Redis (full Scheduler). Both also
// implement task.Task, so they belong in a boot.Run builder or task.Group.
// Start blocks until Stop; cancelling Start's context without Stop leaves the
// runtime live.
//
// # Usage
//
// Register jobs and handlers before Start, then run the driver as a task:
//
//	import (
//		"context"
//		"errors"
//
//		"github.com/go-sphere/sphere/scheduler"
//		"github.com/go-sphere/sphere/scheduler/cron"
//	)
//
//	s, err := cron.NewScheduler(cron.Config{})
//	if err != nil {
//		return err
//	}
//	err = s.Register("cleanup", "@every 1h", func(ctx context.Context) error {
//		return nil
//	})
//	if errors.Is(err, scheduler.ErrDuplicateName) {
//		// "cleanup" was registered twice
//	}
//	// Pass s to task.NewGroup or boot.NewApplication; its Stop drains jobs.
//
// # Contract
//
// Neither driver coordinates periodic jobs across processes. N replicas run
// each periodic job N times per tick. Run the scheduler as a single replica,
// or make handlers idempotent. Enqueue/Handle have no such restriction: the
// queue delivers each task to one consumer.
//
// Handlers are routed by exact kind, not prefix. A kind with no handler is a
// failure so asynq can retry or archive. Register or Handle after Start
// returns [ErrAfterStart]; after Close, [ErrClosed]. Duplicate names return
// [ErrDuplicateName]. Unregister of an unknown name is a no-op. Wrap custom
// handlers with [RecoverHandler] or [RecoverPayloadHandler] to turn panics
// into errors; both drivers already do this for registered handlers.
package scheduler
