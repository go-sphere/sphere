package boot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"time"

	"github.com/go-sphere/sphere/core/safe"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/log"
)

func run(ctx context.Context, t task.Task, options *options) error {
	if ctx.Err() != nil {
		// Shutdown was requested before anything started. The builder has
		// already acquired its resources, so the after-stop hooks still run.
		log.Info("Context cancelled before start; skipping start")
		hookCtx, hookCancel := newShutdownContext(options.shutdownTimeout)
		defer hookCancel()
		afterErr := runHooks(hookCtx, options.afterStop, "afterStop")
		syncLoggerBackend(options.loggerBackend)
		if afterErr != nil {
			return fmt.Errorf("after stop hooks: %w", afterErr)
		}
		return nil
	}
	if err := runHooks(ctx, options.beforeStart, "beforeStart"); err != nil {
		startErr := fmt.Errorf("before start hooks failed: %w", err)
		// After-stop hooks release what the builder acquired (Wire-owned
		// clients, the logger), which a failed readiness check does not undo.
		hookCtx, hookCancel := newShutdownContext(options.shutdownTimeout)
		defer hookCancel()
		afterErr := runHooks(hookCtx, options.afterStop, "afterStop")
		syncLoggerBackend(options.loggerBackend)
		if afterErr != nil {
			return errors.Join(startErr, fmt.Errorf("after stop hooks: %w", afterErr))
		}
		return startErr
	}

	// The task runs under a context detached from the caller's cancellation. A
	// cancelled parent is a shutdown request, handled below like a signal
	// (before-stop hooks, then Stop with its budget); handing it to Start would
	// tear the members down at once, before the hooks and without draining.
	parentDone := ctx.Done()
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()

	var quit chan os.Signal
	if len(options.signals) > 0 {
		quit = make(chan os.Signal, 1)
		signal.Notify(quit, options.signals...)
		defer signal.Stop(quit)
	}

	taskID := safeTaskIdentifier(t)

	// Start a task in a goroutine
	startErr := make(chan error, 1)
	go func() {
		defer close(startErr)
		defer func() {
			if r := recover(); r != nil {
				safe.LogRecovered(taskID, r)
				if err, ok := r.(error); ok {
					startErr <- fmt.Errorf("task panic: %w", err)
				} else {
					startErr <- fmt.Errorf("task panic: %v", r)
				}
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
		// Hand later signals back to the OS default so a second one can
		// terminate a shutdown that hangs.
		signal.Stop(quit)
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
	case <-parentDone:
		shutdownReason = "context cancelled"
		log.Info("Context cancelled")
	}

	log.Infof("Initiating shutdown due to: %s", shutdownReason)

	// Before-stop hooks get their own shutdown window, so a hanging hook
	// cannot stall shutdown indefinitely or eat into Task.Stop's budget.
	hookCtx, hookCancel := newShutdownContext(options.shutdownTimeout)
	defer hookCancel()

	var errs []error
	if err := runHooks(hookCtx, options.beforeStop, "beforeStop"); err != nil {
		errs = append(errs, fmt.Errorf("before stop hooks: %w", err))
	}

	// Stop before cancelling the run ctx so a Group can apply shutdownCtx to
	// member Stop. cancel() afterwards unblocks a leaf Start that only waits
	// on the parent ctx (Stop does not have to).
	shutdownCtx, shutdownCancel := newShutdownContext(options.shutdownTimeout)
	defer shutdownCancel()

	stopErr := stopTask(t, shutdownCtx, taskID)
	if stopErr != nil {
		errs = append(errs, fmt.Errorf("task stop: %w", stopErr))
	}
	cancel()

	if startError == nil {
		if err := joinStartResult(startErr, shutdownCtx); err != nil {
			startError = err
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
	syncLoggerBackend(options.loggerBackend)

	// A Group whose Start already returned reports that same result from Stop.
	// Joining it again as "task start" duplicated the tree in the process error.
	if startError != nil && !errors.Is(stopErr, startError) {
		errs = append(errs, fmt.Errorf("task start: %w", startError))
	}

	return errors.Join(errs...)
}

func safeTaskIdentifier(t task.Task) (id string) {
	if t == nil {
		return "task"
	}
	v := reflect.ValueOf(t)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		if v.IsNil() {
			return "task"
		}
	}
	defer func() {
		if recover() != nil || id == "" {
			id = "task"
		}
	}()
	return t.Identifier()
}

func stopTask(t task.Task, ctx context.Context, taskID string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			safe.LogRecovered(taskID, r)
			if recErr, ok := r.(error); ok {
				err = fmt.Errorf("task panic: %w", recErr)
			} else {
				err = fmt.Errorf("task panic: %v", r)
			}
		}
	}()
	return t.Stop(ctx)
}

// Run configures build-time facilities such as logging, builds an Application
// from conf, and drives its lifecycle: before-start hooks, Start, wait for a
// shutdown signal or task exit, before-stop hooks, Stop, after-stop hooks.
// When a before-start hook fails, Start and Stop are skipped but the after-stop
// hooks still run. A builder that returns a nil Application without an error
// is reported as a build failure.
//
// Run uses context.Background(); use [RunContext] to also stop on a context.
// It stops on SIGTERM, SIGQUIT, or SIGINT (override with WithShutdownSignals)
// or when Start returns. Passing no signals to WithShutdownSignals disables boot's signal
// handling rather than subscribing to every signal. Once the first shutdown
// signal arrives, Run stops handling signals, so a second one gets the
// operating system's default action (for SIGINT and SIGTERM, terminating the
// process) even while shutdown is still in progress.
//
// A clean signal-triggered shutdown returns nil. It returns non-nil when a hook,
// a task's Stop, or a task's Start reports a failure, including failures that
// occur while the application is already shutting down.
//
// If the task is a Group that has already finished, Stop returns the same result
// Start did; that error is reported once, not joined a second time as a start error.
func Run[T any](conf *T, builder func(*T) (*Application, error), options ...Option) error {
	return RunContext(context.Background(), conf, builder, options...)
}

// RunContext is [Run] with a parent context whose cancellation requests a
// shutdown, for tests and for programs that embed the application.
//
// Cancelling ctx is handled like a shutdown signal: before-stop hooks, Stop
// bounded by WithShutdownTimeout, after-stop hooks, and nil for a clean
// shutdown. The tasks' Start context is not cancelled by ctx; it is cancelled
// after Stop returns, so in-flight work drains under the shutdown budget
// rather than being cut off. Signal handling stays active alongside ctx; pass
// WithShutdownSignals() with no signals to stop on ctx alone.
//
// A ctx that is already done when RunContext is called returns nil without
// calling builder. One that ends after the build but before Start skips
// Start and Stop and runs the after-stop hooks. The before-build and
// before-start hooks receive ctx; builder takes no context and is not
// interrupted. A nil ctx is treated as context.Background().
func RunContext[T any](ctx context.Context, conf *T, builder func(*T) (*Application, error), options ...Option) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return nil
	}
	opts := newOptions(options...)
	initLoggerBackend(opts.loggerBackend)
	if err := runHooks(ctx, opts.beforeBuild, "beforeBuild"); err != nil {
		buildErr := fmt.Errorf("before build hooks failed: %w", err)
		return handleBuildFailure(ctx, opts, buildErr)
	}

	app, err := callBuilder(conf, builder)
	if err != nil {
		buildErr := fmt.Errorf("failed to build application: %w", err)
		return handleBuildFailure(ctx, opts, buildErr)
	}
	if app == nil {
		buildErr := fmt.Errorf("failed to build application: %w", errors.New("builder returned a nil application"))
		return handleBuildFailure(ctx, opts, buildErr)
	}
	return run(ctx, app, opts)
}

func callBuilder[T any](conf *T, builder func(*T) (*Application, error)) (app *Application, err error) {
	if builder == nil {
		return nil, errors.New("builder is nil")
	}
	defer func() {
		if r := recover(); r != nil {
			safe.LogRecovered("builder", r)
			if recErr, ok := r.(error); ok {
				err = fmt.Errorf("builder panic: %w", recErr)
			} else {
				err = fmt.Errorf("builder panic: %v", r)
			}
			app = nil
		}
	}()
	return builder(conf)
}

func handleBuildFailure(ctx context.Context, opts *options, buildErr error) error {
	cleanupErr := runHooks(ctx, opts.afterBuildFail, "afterBuildFail")
	syncLoggerBackend(opts.loggerBackend)
	if cleanupErr != nil {
		return errors.Join(buildErr, fmt.Errorf("after build failure hooks: %w", cleanupErr))
	}
	return buildErr
}

// joinStartResult waits for the Start goroutine after Stop. When Stop used
// the whole shutdown budget, Start may still be finishing remaining Group
// stages (a staged closer, logger flush) whose Stop calls already received
// the expired ctx. A short fresh bound lets those return instead of racing
// the process exit. A Start that never returns is abandoned when the bound
// fires; Stop already reported the deadline.
func joinStartResult(startErr <-chan error, shutdownCtx context.Context) error {
	waitCtx := shutdownCtx
	waitCancel := func() {}
	if shutdownCtx.Err() != nil {
		waitCtx, waitCancel = context.WithTimeout(context.Background(), afterStopFallbackTimeout)
	}
	defer waitCancel()

	select {
	case err, ok := <-startErr:
		if ok {
			return err
		}
		return nil
	case <-waitCtx.Done():
		// Start is being abandoned rather than joined. Any error it reports from
		// here on is lost, so say so: otherwise a task that failed during a
		// shutdown that Stop did not report as failed leaves Run returning nil.
		log.Warn("task start result unavailable after shutdown; abandoning wait")
		return nil
	}
}

// newShutdownContext builds the context bounding graceful shutdown. A
// non-positive timeout disables the bound rather than expiring immediately.
func newShutdownContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), timeout)
}
