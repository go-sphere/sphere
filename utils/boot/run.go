package boot

import (
	"context"
	"errors"
	"fmt"
	"github.com/TBXark/sphere/utils/task"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

type Options struct {
	shutdownTimeout time.Duration
	beforeStart     []func()
	beforeStop      []func()
}

func newOptions(opts ...Option) *Options {
	opt := &Options{
		shutdownTimeout: 30 * time.Second,
	}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

type Option func(*Options)

func WithShutdownTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.shutdownTimeout = d
	}
}

func WithBeforeStart(f func()) Option {
	return func(o *Options) {
		o.beforeStart = append(o.beforeStart, f)
	}
}

func WithBeforeStop(f func()) Option {
	return func(o *Options) {
		o.beforeStop = append(o.beforeStop, f)
	}
}

func WithLoggerInit(ver string, conf *log.Options) Option {
	return func(o *Options) {
		o.beforeStart = append(o.beforeStart, func() {
			log.Init(conf, logfields.String("version", ver))
		})
		o.beforeStop = append(o.beforeStop, func() {
			_ = log.Sync()
		})
	}
}

func run(ctx context.Context, task task.Task, options ...Option) error {
	opts := newOptions(options...)

	// Execute before start hooks
	for _, f := range opts.beforeStart {
		f()
	}

	// Create root context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Listen for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// Start application
	errChan := make(chan error, 1)
	go func() {
		errChan <- task.Start(ctx)
	}()

	// Wait for shutdown signal or application error
	var errs []error
	select {
	case sig := <-quit:
		log.Infof("Received shutdown signal: %v", sig)
		cancel() // Trigger application shutdown
	case err := <-errChan:
		if err != nil {
			log.Error("Application error", logfields.Error(err))
			errs = append(errs, fmt.Errorf("application error: %w", err))
			cancel() // Ensure context is canceled
		}
	}

	// Execute before stop hooks
	for _, f := range opts.beforeStop {
		f()
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), opts.shutdownTimeout)
	defer shutdownCancel()
	if err := task.Stop(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown error: %w", err))
	}
	return errors.Join(errs...)
}

func Run[T any](conf *T, builder func(*T) (*Application, error), options ...Option) error {
	// Create application
	app, err := builder(conf)
	if err != nil {
		return fmt.Errorf("failed to build application: %w", err)
	}

	// Run application
	return run(context.Background(), app, options...)
}
