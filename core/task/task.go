// Package task is a blocking Start/Stop lifecycle for servers, workers, and
// one-shot jobs, plus the combinators that run them.
//
// Start may block until shutdown (an HTTP listener). The context is a
// best-effort cancel: listeners typically ignore it. Stop must unblock Start,
// and must be safe before Start, after Start has returned, concurrently, and
// when Start failed. Group and Manager always call Stop for a task whose Start
// was invoked.
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
package task

import (
	"context"
)

// Task is a lifecycle-managed component: a server, worker, or other
// background operation. Group and Manager always call Stop for a task whose
// Start was invoked, including when Start returned on its own.
type Task interface {
	// Identifier returns a unique identifier for this task, used in logs.
	Identifier() string

	// Start begins the task's operation. It may block until the task is
	// shutting down (an HTTP listener, a ticker loop). The context is a
	// best-effort cancellation signal: honour it when possible, but HTTP-style
	// listeners typically ignore it and only return after Stop has closed the
	// listener. Stop is therefore the signal that must be able to unblock Start.
	// Returns an error if the task fails to start.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the task with the given context.
	// The context may have a deadline for shutdown completion.
	// Stop must be idempotent and safe to call concurrently.
	// Stop must also be safe when Start failed, returned already, or has not completed initialization.
	// Returns an error if the task fails to stop cleanly.
	Stop(ctx context.Context) error
}
