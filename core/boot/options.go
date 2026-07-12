package boot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/zapx"
)

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
func WithShutdownTimeout(d time.Duration) Option {
	return func(o *options) {
		o.shutdownTimeout = d
	}
}

// WithShutdownSignals configures which OS signals will trigger application shutdown.
// Replaces the default signals (SIGTERM, SIGQUIT, SIGINT) with the provided ones.
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
		if err := f(ctx); err != nil {
			log.Errorf("Hook %s[%d] failed: %v", hookType, i, err)
			errs = append(errs, fmt.Errorf("%s hook[%d]: %w", hookType, i, err))
		}
	}
	return errors.Join(errs...)
}
