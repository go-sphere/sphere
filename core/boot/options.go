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

// Hook defines a function that can be executed at various lifecycle stages of the application.
// It receives a context and returns an error if the hook execution fails.
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

// Option defines a configuration function that modifies application runtime options.
type Option func(*options)

// WithShutdownTimeout configures the maximum duration to wait for graceful shutdown.
// If the timeout is exceeded, the application will be forcefully terminated.
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
// handling: Run then stops only when the task returns or the parent context
// is cancelled. The operating system's default handling of those signals
// still applies (SIGINT typically terminates the process).
//
// An empty list does not subscribe to every signal. signal.Notify with no
// arguments would, including SIGURG used by the Go runtime.
func WithShutdownSignals(sigs ...os.Signal) Option {
	return func(o *options) {
		o.signals = sigs
	}
}

// AddBeforeStart adds a hook that will be executed before the application starts.
// These hooks run sequentially and any failure will prevent the application from starting.
func AddBeforeStart(f Hook) Option {
	return func(o *options) {
		o.beforeStart = append(o.beforeStart, f)
	}
}

// AddBeforeStop adds a hook that will be executed before the application begins shutdown.
// These hooks run after a shutdown signal is received but before stopping tasks.
func AddBeforeStop(f Hook) Option {
	return func(o *options) {
		o.beforeStop = append(o.beforeStop, f)
	}
}

// AddAfterStop adds a hook that will be executed after the application has stopped.
// These hooks run after all tasks have been stopped and are useful for cleanup operations.
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
