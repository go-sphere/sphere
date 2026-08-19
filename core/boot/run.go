package boot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/go-sphere/sphere/core/safe"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/log"
)

func run(ctx context.Context, t task.Task, options *options) error {
	// Execute before start hooks
	if err := runHooks(ctx, options.beforeStart, "beforeStart"); err != nil {
		return fmt.Errorf("before start hooks failed: %w", err)
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, options.signals...)
	defer signal.Stop(quit)

	// Start a task in a goroutine
	startErr := make(chan error, 1)
	go func() {
		defer close(startErr) // 确保 channel 被关闭
		defer func() {
			if r := recover(); r != nil {
				safe.LogRecovered(t.Identifier(), r)
				startErr <- fmt.Errorf("task panic: %v", r)
			}
		}()
		if err := t.Start(ctx); err != nil {
			startErr <- err
		}
	}()

	// Wait for a shutdown signal or task error
	var shutdownReason string
	var startError error

	select {
	case sig := <-quit:
		shutdownReason = fmt.Sprintf("signal %v", sig)
		log.Infof("Received shutdown signal: %v", sig)
	case err, ok := <-startErr:
		if ok && err != nil {
			startError = err
			shutdownReason = "task error"
			log.Error("Task start error", log.Err(err))
		} else {
			shutdownReason = "task completed"
			log.Info("Task completed normally")
		}
	case <-ctx.Done():
		shutdownReason = "context cancelled"
		log.Info("Context cancelled")
	}

	log.Infof("Initiating shutdown due to: %s", shutdownReason)

	// Execute before stop hooks
	var errs []error
	if err := runHooks(ctx, options.beforeStop, "beforeStop"); err != nil {
		errs = append(errs, fmt.Errorf("before stop hooks: %w", err))
	}

	// Cancel context to signal shutdown
	cancel()

	// Graceful shutdown with timeout. A non-positive timeout means unbounded,
	// consistent with task.WithCleanupTimeout; handing it to WithTimeout would
	// produce an already-expired context and skip the graceful phase outright.
	shutdownCtx, shutdownCancel := newShutdownContext(options.shutdownTimeout)
	defer shutdownCancel()

	if err := t.Stop(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("task stop: %w", err))
	}

	// Execute after stop hooks
	if err := runHooks(shutdownCtx, options.afterStop, "afterStop"); err != nil {
		errs = append(errs, fmt.Errorf("after stop hooks: %w", err))
	}

	// Include start error if any
	if startError != nil {
		errs = append(errs, fmt.Errorf("task start: %w", startError))
	}

	return errors.Join(errs...)
}

// Run executes an application built from the provided configuration using the builder function.
// It handles the complete application lifecycle including startup, signal handling, and graceful shutdown.
// The builder function receives the configuration and should return a properly initialized Application.
// Returns an error if the application fails to build, start, or encounters issues during execution.
//
// A clean signal-triggered shutdown returns nil. It returns non-nil when a hook,
// a task's Stop, or a task's Start reports a failure — including failures that
// occur while the application is already shutting down, which earlier releases
// dropped. Programs that exit non-zero on a non-nil result should be prepared for
// shutdown-time task errors to become visible as failed exits.
func Run[T any](conf *T, builder func(*T) (*Application, error), options ...Option) error {
	// Create application
	app, err := builder(conf)
	if err != nil {
		return fmt.Errorf("failed to build application: %w", err)
	}
	opts := newOptions(options...)
	// Run application
	return run(context.Background(), app, opts)
}

// newShutdownContext builds the context bounding graceful shutdown. A
// non-positive timeout disables the bound rather than expiring immediately.
func newShutdownContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), timeout)
}
