package boot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/TBXark/sphere/core/task"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

func runWithContext(ctx context.Context, t task.Task, options ...Option) error {
	opts := newOptions(options...)

	// Execute before start hooks
	if err := runHooks(opts.beforeStart, "beforeStart"); err != nil {
		return fmt.Errorf("before start hooks failed: %w", err)
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, opts.signals...)
	defer signal.Stop(quit)

	// Start a task in a goroutine
	startErr := make(chan error, 1)
	go func() {
		defer close(startErr) // 确保 channel 被关闭
		defer func() {
			if r := recover(); r != nil {
				log.Errorw("Task panic",
					logfields.String("task", t.Identifier()),
					logfields.Any("recover", r),
				)
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
			log.Error("Task start error", logfields.Error(err))
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
	if err := runHooks(opts.beforeStop, "beforeStop"); err != nil {
		errs = append(errs, fmt.Errorf("before stop hooks: %w", err))
	}

	// Cancel context to signal shutdown
	cancel()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), opts.shutdownTimeout)
	defer shutdownCancel()

	if err := t.Stop(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("task stop: %w", err))
	}

	// Execute after stop hooks
	if err := runHooks(opts.afterStop, "afterStop"); err != nil {
		errs = append(errs, fmt.Errorf("after stop hooks: %w", err))
	}

	// Include start error if any
	if startError != nil {
		errs = append(errs, fmt.Errorf("task start: %w", startError))
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
	return runWithContext(context.Background(), app, options...)
}
