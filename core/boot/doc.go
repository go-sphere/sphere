// Package boot runs a task.Task as a process: OS signals, lifecycle hooks,
// and a shutdown deadline.
//
// Entry points: [Run] drives an [Application] built by [NewApplication],
// [NewStagedApplication], or [NewApplicationFromGroup]; [Option] values such
// as [WithShutdownTimeout], [WithLoggerBackend], and [AddAfterStop] customize
// it. [DefaultConfigParser] and [InitTimezone] are optional main helpers.
//
// # Usage
//
//	import (
//		"context"
//		"os"
//
//		"github.com/go-sphere/sphere/core/boot"
//	)
//
//	func main() {
//		conf := &Config{}
//		err := boot.Run(conf, func(c *Config) (*boot.Application, error) {
//			httpSrv := newHTTPTask(c) // any task.Task
//			return boot.NewApplication(httpSrv), nil
//		}, boot.AddAfterStop(func(ctx context.Context) error {
//			return db.Close() // clients that are not Tasks close after Stop
//		}))
//		if err != nil {
//			os.Exit(1)
//		}
//	}
//
// Importing this package has no side effects. The process timezone is the
// host's unless main calls InitTimezone, e.g. InitTimezone(DefaultTimezone).
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
// Run waits for SIGTERM, SIGQUIT, or SIGINT (the WithShutdownSignals
// default), then Stop. Concurrent stop of HTTP and a
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
// already-expired one. The same fallback bounds waiting for Start so a
// staged Group can still Stop earlier stages after a last-stage timeout.
//
// WithShutdownSignals() with no arguments disables boot's signal handling
// rather than subscribing to every signal.
package boot
