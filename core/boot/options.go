package boot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/go-sphere/sphere/core/safe"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/zapx"
)

// afterStopFallbackTimeout is used when Stop consumed the whole shutdown
// budget, so after-stop hooks are not handed an already-expired context.
const afterStopFallbackTimeout = 2 * time.Second

// Hook is a lifecycle callback. All hooks in a phase run even if an earlier
// one failed; Run joins the errors.
type Hook = func(context.Context) error

type options struct {
	shutdownTimeout time.Duration
	beforeStart     []Hook
	beforeStop      []Hook
	afterStop       []Hook
	signals         []os.Signal
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		shutdownTimeout: 30 * time.Second,
		signals:         []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// Option configures Run.
type Option func(*options)

// WithShutdownTimeout bounds the context passed to Task.Stop during graceful
// shutdown. It does not kill the process or abort goroutines that ignore
// context: when the deadline expires, Stop is asked to return, after-stop
// hooks still run, and Run then returns.
//
// A non-positive duration means no limit, matching task.WithCleanupTimeout and
// task.WithManagerCleanupTimeout. Passing it straight through instead produced a
// context that was already expired when shutdown began, so the graceful phase
// was skipped entirely and even the after-stop hooks ran on a dead context —
// the exact opposite of what "no timeout" reads like next to the other two.
//
// For an Application (a Group), this deadline is also applied to each member
// Stop, capped by the group's WithCleanupTimeout (default 30s as well).
func WithShutdownTimeout(d time.Duration) Option {
	return func(o *options) {
		o.shutdownTimeout = d
	}
}

// WithShutdownSignals replaces the default shutdown signals
// (SIGTERM, SIGQUIT, SIGINT). Passing no signals disables boot's signal
// handling: the exported Run then stops only when the task returns. The
// operating system's default handling of those signals still applies
// (SIGINT typically terminates the process).
//
// An empty list does not subscribe to every signal. signal.Notify with no
// arguments would, including SIGURG used by the Go runtime.
func WithShutdownSignals(sigs ...os.Signal) Option {
	return func(o *options) {
		o.signals = sigs
	}
}

// AddBeforeStart appends a hook that runs after the builder returns and before
// Task.Start. Hooks run in registration order; a failure does not skip later
// hooks. Run joins every error and returns before Start if any hook failed.
func AddBeforeStart(f Hook) Option {
	return func(o *options) {
		o.beforeStart = append(o.beforeStart, f)
	}
}

// AddBeforeStop appends a hook that runs after a shutdown is requested (signal
// or task exit) and before Task.Stop. Hooks run in registration order; a
// failure does not skip later hooks.
func AddBeforeStop(f Hook) Option {
	return func(o *options) {
		o.beforeStop = append(o.beforeStop, f)
	}
}

// AddAfterStop appends a hook that runs after Task.Stop returns. Use it to
// close Wire-owned clients (sql.DB, Redis) that are not themselves Tasks.
// Hooks run in registration order; a failure does not skip later hooks. When
// Stop consumes the whole shutdown budget, these hooks receive a short fresh
// context instead of an already-expired one.
func AddAfterStop(f Hook) Option {
	return func(o *options) {
		o.afterStop = append(o.afterStop, f)
	}
}

// slogBackend is an optional capability for backends that can expose a
// *slog.Logger, allowing WithLoggerBackend to also route the standard library's
// slog output through the configured backend.
type slogBackend interface {
	SlogLogger(options ...log.Option) *slog.Logger
}

// WithLoggerBackend configures automatic logger initialization with the provided backend.
// It installs the backend as the global logger before start and syncs it after stop.
// The caller constructs the backend (for example zapx.NewBackend(conf)), so the
// lifecycle implementation works with any logging driver. When the backend also
// implements SlogLogger, it is registered as the default slog handler.
func WithLoggerBackend(backend log.Backend) Option {
	return func(o *options) {
		o.beforeStart = append(o.beforeStart, func(context.Context) error {
			log.InitWithBackends(backend)
			if sb, ok := backend.(slogBackend); ok {
				slog.SetDefault(sb.SlogLogger())
			}
			return nil
		})
		o.afterStop = append(o.afterStop, func(context.Context) error {
			_ = log.Sync()
			return nil
		})
	}
}

// WithLoggerInit configures the legacy zap logger integration. The version is
// attached to every log entry as a fixed "version" attribute.
//
// Deprecated: use WithLoggerBackend with an explicitly constructed backend.
func WithLoggerInit(version string, conf zapx.Config) Option {
	return WithLoggerBackend(zapx.NewBackend(
		conf,
		log.WithAttrs(map[string]any{"version": version}),
	))
}

func runHooks(ctx context.Context, hooks []Hook, hookType string) error {
	var errs []error
	for i, f := range hooks {
		err := func() (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					safe.LogRecovered(fmt.Sprintf("hook %s[%d]", hookType, i), rec)
					err = fmt.Errorf("panic: %v", rec)
				}
			}()
			return f(ctx)
		}()
		if err != nil {
			log.Errorf("Hook %s[%d] failed: %v", hookType, i, err)
			errs = append(errs, fmt.Errorf("%s hook[%d]: %w", hookType, i, err))
		}
	}
	return errors.Join(errs...)
}
