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
	if err := runHooks(ctx, options.beforeStart, "beforeStart"); err != nil {
		return fmt.Errorf("before start hooks failed: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	quit := make(chan os.Signal, 1)
	if len(options.signals) > 0 {
		signal.Notify(quit, options.signals...)
		defer signal.Stop(quit)
	}

	// Start a task in a goroutine
	startErr := make(chan error, 1)
	go func() {
		defer close(startErr)
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

	var errs []error
	if err := runHooks(ctx, options.beforeStop, "beforeStop"); err != nil {
		errs = append(errs, fmt.Errorf("before stop hooks: %w", err))
	}

	// Stop before cancelling the run ctx so a Group can apply shutdownCtx to
	// member Stop. cancel() afterwards unblocks a leaf Start that only waits
	// on the parent ctx (Stop does not have to).
	shutdownCtx, shutdownCancel := newShutdownContext(options.shutdownTimeout)
	defer shutdownCancel()

	stopErr := t.Stop(shutdownCtx)
	if stopErr != nil {
		errs = append(errs, fmt.Errorf("task stop: %w", stopErr))
	}
	cancel()

	if startError == nil {
		select {
		case err, ok := <-startErr:
			if ok && err != nil {
				startError = err
			}
		case <-shutdownCtx.Done():
		}
	}

	afterCtx := shutdownCtx
	afterCancel := func() {}
	if shutdownCtx.Err() != nil {
		afterCtx, afterCancel = context.WithTimeout(context.Background(), afterStopFallbackTimeout)
	}
	defer afterCancel()

	if err := runHooks(afterCtx, options.afterStop, "afterStop"); err != nil {
		errs = append(errs, fmt.Errorf("after stop hooks: %w", err))
	}

	// A Group whose Start already returned reports that same result from Stop.
	// Joining it again as "task start" duplicated the tree in the process error.
	if startError != nil && !errors.Is(stopErr, startError) {
		errs = append(errs, fmt.Errorf("task start: %w", startError))
	}

	return errors.Join(errs...)
}

// Run configures build-time facilities such as logging, builds an Application
// from conf, and drives its lifecycle: before-start hooks, Start, wait for a
// shutdown signal or task exit, before-stop hooks, Stop, after-stop hooks.
//
// The exported Run always uses context.Background(); it stops on SIGTERM,
// SIGQUIT, or SIGINT (override with WithShutdownSignals) or when Start
// returns. Passing no signals to WithShutdownSignals disables boot's signal
// handling rather than subscribing to every signal.
//
// A clean signal-triggered shutdown returns nil. It returns non-nil when a hook,
// a task's Stop, or a task's Start reports a failure — including failures that
// occur while the application is already shutting down, which earlier releases
// dropped. Programs that exit non-zero on a non-nil result should be prepared for
// shutdown-time task errors to become visible as failed exits.
//
// If the task is a Group that has already finished, Stop returns the same result
// Start did; that error is reported once, not joined a second time as a start error.
func Run[T any](conf *T, builder func(*T) (*Application, error), options ...Option) error {
	opts := newOptions(options...)
	ctx := context.Background()
	if err := runHooks(ctx, opts.beforeBuild, "beforeBuild"); err != nil {
		buildErr := fmt.Errorf("before build hooks failed: %w", err)
		if cleanupErr := runHooks(ctx, opts.afterBuildFail, "afterBuildFail"); cleanupErr != nil {
			return errors.Join(buildErr, fmt.Errorf("after build failure hooks: %w", cleanupErr))
		}
		return buildErr
	}

	app, err := builder(conf)
	if err != nil {
		buildErr := fmt.Errorf("failed to build application: %w", err)
		if cleanupErr := runHooks(ctx, opts.afterBuildFail, "afterBuildFail"); cleanupErr != nil {
			return errors.Join(buildErr, fmt.Errorf("after build failure hooks: %w", cleanupErr))
		}
		return buildErr
	}
	return run(ctx, app, opts)
}

// newShutdownContext builds the context bounding graceful shutdown. A
// non-positive timeout disables the bound rather than expiring immediately.
func newShutdownContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), timeout)
}
