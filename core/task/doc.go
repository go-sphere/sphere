// Package task is a blocking Start/Stop lifecycle for servers, workers, and
// one-shot jobs, plus the combinators that run them.
//
// Start may block until shutdown (an HTTP listener). The context is a
// best-effort cancel: listeners typically ignore it. Stop must unblock Start,
// and must be safe before Start, after Start has returned, concurrently, and
// when Start failed. Group and Manager always call Stop for a task whose Start
// was invoked.
//
// Entry points: [NewFunc] adapts callbacks to [Task]; [NewGroup] and
// [NewStagedGroup] build a [Group] that runs tasks to completion or until
// stopped; [NewManager] supervises named tasks added at runtime.
//
// Two recipes cover the intended use:
//
//   - One-shot (migrate, warmup, script): put jobs in a Group and call Start.
//     When every Start returns, the group stops those tasks and Start returns.
//     boot.Run is optional; it adds signals so a long job can be interrupted.
//
//   - Process (HTTP and companions): put blocking servers in a Group, usually
//     via boot.Run and boot.NewApplication. boot waits for a signal or parent
//     cancel, then Stop. Database clients owned by Wire are not Tasks — close
//     them in a boot after-stop hook, or in the injector's cleanup after Run
//     returns. Use NewStagedGroup only when one task's Stop tears down something
//     another task still uses while draining; last stage stops first.
//
// An external Group.Stop(ctx) with a deadline bounds member Stop to
// min(that deadline, WithCleanupTimeout). Internal teardown (failure, parent
// cancel, natural complete) uses WithCleanupTimeout alone (default 30s).
// boot.WithShutdownTimeout is that external deadline when using Run.
//
// Manager is a supervisor for named tasks started later at runtime. It is not
// the process runner; HTTP servers belong in a Group.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/core/task"
//
//	migrate := task.NewFunc("migrate", func(ctx context.Context) error {
//		return runMigrations(ctx)
//	}, nil)
//	if err := task.NewGroup(migrate).Start(ctx); err != nil {
//		return err // joined Start and Stop errors of the members
//	}
//
// For a process, pass the tasks to boot.NewApplication (package
// github.com/go-sphere/sphere/core/boot) and let boot.Run drive Start and Stop.
// For runtime-managed workers:
//
//	m := task.NewManager()
//	if err := m.StartTask(ctx, "sync", worker); err != nil {
//		return err
//	}
//	defer func() { _ = m.StopAll(ctx) }()
package task
