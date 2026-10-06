package test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
	"github.com/go-sphere/sphere/scheduler"
)

func TestSchedulerLifecycleContract(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			tasktest.AssertLifecycleContract(t, func() task.Task {
				runtime := factory.new(t)
				tk, ok := runtime.(task.Task)
				if !ok {
					t.Fatalf("%s scheduler does not implement task.Task", factory.name)
				}
				return tk
			})
		})
	}
}

// TestSchedulerTightShutdownTimeoutAndRecovery tests that calling Stop with an expired or
// very tight deadline does not deadlock or leave the scheduler in a permanently broken state.
func TestSchedulerTightShutdownTimeoutAndRecovery(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			taskStarted := make(chan struct{})
			taskRelease := make(chan struct{})
			signalTaskStarted := sync.OnceFunc(func() { close(taskStarted) })

			err := s.Register("long_task", "@every 1s", func(ctx context.Context) error {
				signalTaskStarted()
				select {
				case <-taskRelease:
					return nil
				case <-time.After(3 * time.Second):
					return nil
				}
			})
			if err != nil {
				t.Fatalf("register: %v", err)
			}

			startCtx, startCancel := context.WithCancel(context.Background())
			defer startCancel()

			startDone := make(chan error, 1)
			go func() {
				startDone <- s.Start(startCtx)
			}()

			// Wait for task to start running
			waitForChan(t, 3*time.Second, taskStarted)

			// Step 1: Call Stop with an expired context (0 timeout)
			expiredCtx, cancelExpired := context.WithTimeout(context.Background(), 0)
			defer cancelExpired()

			err = s.Stop(expiredCtx)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected DeadlineExceeded on expired Stop, got: %v", err)
			}

			// Step 2: Call Stop with 1ms tight timeout while task is still holding
			tightCtx, cancelTight := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancelTight()

			err = s.Stop(tightCtx)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected DeadlineExceeded on tight Stop, got: %v", err)
			}

			// Step 3: Unblock the task and retry Stop with full timeout: it MUST succeed and cleanly quiesce
			close(taskRelease)

			gracefulCtx, cancelGraceful := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelGraceful()

			err = s.Stop(gracefulCtx)
			if err != nil {
				t.Fatalf("graceful retry Stop failed: %v", err)
			}

			// State must now be closed
			startCancel()
			select {
			case sErr := <-startDone:
				if sErr != nil && !errors.Is(sErr, context.Canceled) && !errors.Is(sErr, scheduler.ErrClosed) {
					t.Fatalf("start returned unexpected error: %v", sErr)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("start failed to terminate after graceful stop")
			}

			// Subsequent calls must report ErrClosed
			if err := s.Register("late", "@every 1s", func(context.Context) error { return nil }); !errors.Is(err, scheduler.ErrClosed) {
				t.Fatalf("expected ErrClosed on register, got: %v", err)
			}
			if err := s.Close(); !errors.Is(err, scheduler.ErrClosed) {
				t.Fatalf("expected ErrClosed on close, got: %v", err)
			}
		})
	}
}

// TestSchedulerStopBeforeStartIsHonoured pins that a Stop arriving before Start
// is recorded rather than dropped, matching task.Group: the later Start returns
// nil promptly without bringing up a runtime, and the scheduler is closed.
func TestSchedulerStopBeforeStartIsHonoured(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			if err := s.Stop(context.Background()); err != nil {
				t.Fatalf("stop before start: %v", err)
			}

			startDone := make(chan error, 1)
			go func() {
				startDone <- s.Start(context.Background())
			}()
			select {
			case err := <-startDone:
				if err != nil {
					t.Fatalf("start after stop = %v, want nil", err)
				}
			case <-time.After(2 * time.Second):
				_ = s.Close()
				t.Fatal("start after stop stayed blocked: the early Stop was dropped")
			}
			if err := s.Register("late", "@every 1s", func(context.Context) error { return nil }); !errors.Is(err, scheduler.ErrClosed) {
				t.Fatalf("register after stop error = %v, want %v", err, scheduler.ErrClosed)
			}
		})
	}
}

// TestSchedulerStopRacingStartIsHonoured runs Start and Stop concurrently with
// no ordering. Whichever wins, Start must return once Stop has returned.
func TestSchedulerStopRacingStartIsHonoured(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			for range 10 {
				s := factory.new(t)
				startDone := make(chan error, 1)
				go func() {
					startDone <- s.Start(context.Background())
				}()
				if err := s.Stop(context.Background()); err != nil {
					t.Fatalf("stop: %v", err)
				}
				select {
				case err := <-startDone:
					if err != nil && !errors.Is(err, context.Canceled) {
						t.Fatalf("start = %v, want nil or context.Canceled", err)
					}
				case <-time.After(2 * time.Second):
					_ = s.Close()
					t.Fatal("start stayed blocked after a concurrent Stop returned")
				}
			}
		})
	}
}
