// Package cron is a scheduler.Cron on robfig/cron/v3. It is not a full
// scheduler.Scheduler: there is no Enqueue or Handle.
//
// [NewScheduler] builds a [Scheduler] from a [Config] (optional seconds field
// and timezone). Jobs run through robfig's SkipIfStillRunning and Recover
// chain; handler errors are logged with the job name. The Scheduler
// implements task.Task: register every job, then run it in task.Group or
// boot.Run, which calls Stop on shutdown.
//
// # Usage
//
//	import (
//		"context"
//
//		"github.com/go-sphere/sphere/scheduler/cron"
//	)
//
//	s, err := cron.NewScheduler(cron.Config{Seconds: true, Timezone: "UTC"})
//	if err != nil {
//		return err
//	}
//	if err := s.Register("report", "0 */5 * * * *", func(ctx context.Context) error {
//		return nil
//	}); err != nil {
//		return err
//	}
//	// Run s as a task (Start blocks; Stop drains running jobs), or call
//	// s.Close() if it was never started.
//
// # Lifecycle
//
// Start blocks until Stop finishes draining. Stop waits for in-flight jobs
// without cancelling their context until its own ctx expires; handler contexts
// are detached from Start's context, so a runner that cancels the parent
// context before calling Stop (task.Group does) still gets a real drain. When
// the last waiting Stop times out, handler contexts are cancelled so jobs can
// abort before the process exits. Cancelling Start's context
// without Stop leaves the cron running. A Stop that arrives before Start is
// honoured: Start then returns nil without starting the cron. Duplicate
// Register returns scheduler.ErrDuplicateName; Register after Start returns
// scheduler.ErrAfterStart; unknown Unregister is a no-op.
package cron
