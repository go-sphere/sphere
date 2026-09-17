// Package tasktest provides reusable test helpers for task.Task: a Fake test
// double and AssertLifecycleContract for a subset of the lifecycle guarantees
// documented on the interface.
//
// AssertLifecycleContract checks that Stop is safe before Start, Stop is
// idempotent, and concurrent Stop does not panic or deadlock. It does not
// assert Stop-after-Start-returned, Stop-when-Start-failed, or that Stop
// unblocks Start without also cancelling ctx. The factory must return a task
// that unblocks on Stop or ctx cancel; a Start that ignores both fails the
// helper at contractTimeout (5s).
package tasktest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
)

// contractTimeout bounds each contract step so a stuck Start/Stop surfaces as a
// deadlock failure instead of hanging the whole test binary.
const contractTimeout = 5 * time.Second

// AssertLifecycleContract exercises Stop-before-Start, idempotent Stop, and
// concurrent Stop against a factory that returns a fresh, not-yet-started
// task on every call. None of those may panic or deadlock. See the package
// comment for what it does not cover.
func AssertLifecycleContract(t *testing.T, newTask func() task.Task) {
	t.Helper()
	if newTask == nil {
		t.Fatal("tasktest: newTask must not be nil")
	}

	// Stop must be safe when Start was never called.
	t.Run("StopWithoutStart", func(t *testing.T) {
		tk := mustNewTask(t, newTask)
		runWithin(t, "Stop before Start", func() {
			_ = tk.Stop(context.Background())
		})
	})

	// Repeated Stop calls must be idempotent and never panic or deadlock.
	t.Run("IdempotentStop", func(t *testing.T) {
		tk := mustNewTask(t, newTask)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		startDone := startInBackground(t, ctx, tk)

		runWithin(t, "first Stop", func() { _ = tk.Stop(context.Background()) })
		runWithin(t, "second Stop", func() { _ = tk.Stop(context.Background()) })
		runWithin(t, "third Stop", func() { _ = tk.Stop(context.Background()) })

		// Release a blocking Start the way a runner would, then confirm it returns.
		cancel()
		awaitStart(t, startDone)
	})

	// Concurrent Stop callers must all return without panic or deadlock.
	t.Run("ConcurrentStop", func(t *testing.T) {
		tk := mustNewTask(t, newTask)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		startDone := startInBackground(t, ctx, tk)

		const stoppers = 8
		panics := make(chan any, stoppers)
		var wg sync.WaitGroup
		wg.Add(stoppers)
		for range stoppers {
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panics <- r
					}
				}()
				_ = tk.Stop(context.Background())
			}()
		}
		runWithin(t, "concurrent Stop", wg.Wait)
		close(panics)
		for r := range panics {
			t.Errorf("tasktest: concurrent Stop panicked: %v", r)
		}

		cancel()
		awaitStart(t, startDone)
	})
}

func mustNewTask(t *testing.T, newTask func() task.Task) task.Task {
	t.Helper()
	tk := newTask()
	if tk == nil {
		t.Fatal("tasktest: newTask returned a nil task")
	}
	return tk
}

// startInBackground calls Start in a goroutine. Start may block until the task's
// context is cancelled, so completion is reported through the returned channel:
// a recovered panic value is sent before the channel is closed. The goroutine
// must not touch t itself — testing.T panics on Log/Error after the test has
// returned, which is reachable when an earlier step already called t.Fatal.
func startInBackground(t *testing.T, ctx context.Context, tk task.Task) <-chan any {
	t.Helper()
	done := make(chan any, 1)
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				done <- r
			}
		}()
		_ = tk.Start(ctx)
	}()
	return done
}

// awaitStart waits for a backgrounded Start to return once its context has been
// cancelled, failing if it never unblocks. A panic recovered in the background
// goroutine is reported here, from the test goroutine.
func awaitStart(t *testing.T, done <-chan any) {
	t.Helper()
	select {
	case r, ok := <-done:
		if ok && r != nil {
			t.Errorf("tasktest: Start panicked: %v", r)
		}
	case <-time.After(contractTimeout):
		t.Fatal("tasktest: Start did not return after context cancellation; possible deadlock")
	}
}

// runWithin runs fn in a goroutine, failing the test if it does not finish
// within contractTimeout (deadlock) or if it panics.
func runWithin(t *testing.T, desc string, fn func()) {
	t.Helper()
	var panicVal any
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { panicVal = recover() }()
		fn()
	}()
	select {
	case <-done:
		if panicVal != nil {
			t.Errorf("tasktest: %s panicked: %v", desc, panicVal)
		}
	case <-time.After(contractTimeout):
		t.Fatalf("tasktest: %s did not complete within %s; possible deadlock", desc, contractTimeout)
	}
}
