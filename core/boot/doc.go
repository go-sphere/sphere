// Package boot runs a task.Task as a process: OS signals, lifecycle hooks,
// and a shutdown deadline.
//
// Importing this package sets time.Local and TZ to Asia/Shanghai in init.
// Call InitTimezone again before Run to override.
//
// Lifecycle is split on purpose. task.Group starts and stops members; Run
// decides when to ask the application to stop (signal or task exit) and runs
// hooks around that. The exported Run always starts from context.Background();
// it does not take a parent context. Stop is idempotent: if the group has
// already stopped — a one-shot job that finished — Run's Stop is a no-op and
// after-stop hooks still run.
//
// # One-shot job
//
//	err := boot.Run(conf, func(c *Conf) (*boot.Application, error) {
//	    return boot.NewApplication(migrateTask), nil
//	})
//
// The job's Start returns, the group stops it (cleanup), Run sees completion
// and exits. A signal during the job also Stop's it.
//
// # HTTP server and infra
//
//	err := boot.Run(conf, func(c *Conf) (*boot.Application, error) {
//	    return boot.NewApplication(httpTask, consumerTask), nil
//	}, boot.WithLoggerBackend(backend))
//
// Run waits for SIGINT/SIGTERM, then Stop. Concurrent stop of HTTP and a
// consumer is fine while sql.DB stays open. Close Wire-owned clients after
// every Task.Stop: either AddAfterStop, or:
//
//	app, cleanup, err := Initialize(conf) // wire injector
//	if err != nil { ... }
//	defer cleanup()
//	err = boot.Run(conf, func(*Conf) (*boot.Application, error) {
//	    return app, nil
//	})
//
// Use NewStagedApplication when one task's Stop tears down something another
// task still uses while draining (last stage stops first).
//
// # Timeouts
//
// WithShutdownTimeout (default 30s) bounds Stop, including each member Stop
// of an Application, capped by task.WithCleanupTimeout (also default 30s).
// After-stop hooks share that context when Stop finishes early; if Stop
// consumes the whole budget they get a short fresh context instead of an
// already-expired one.
//
// WithShutdownSignals() with no arguments disables boot's signal handling
// rather than subscribing to every signal.
package boot
