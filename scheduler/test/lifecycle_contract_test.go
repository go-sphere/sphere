package test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

// TestSchedulerRapidLifecycleCycles tests rapid Start, Stop, and Close cycles
// across multiple iterations to ensure no deadlocks, race conditions, or hanging goroutines.
func TestSchedulerRapidLifecycleCycles(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			const iterations = 15
			for iter := range iterations {
				s := factory.new(t)
				var count atomic.Int32
				err := s.Register(fmt.Sprintf("job_%d", iter), "@every 1s", func(context.Context) error {
					count.Add(1)
					return nil
				})
				if err != nil {
					t.Fatalf("iter %d register: %v", iter, err)
				}

				// Pattern 1: Start and immediately Stop
				ctx, cancel := context.WithCancel(context.Background())
				startDone := make(chan error, 1)
				go func() {
					startDone <- s.Start(ctx)
				}()

				// Rapid stop with varied timeouts
				stopTimeout := time.Duration((iter%5)*50) * time.Millisecond
				if stopTimeout == 0 {
					stopTimeout = 50 * time.Millisecond
				}
				stopCtx, stopCancel := context.WithTimeout(context.Background(), stopTimeout)
				_ = s.Stop(stopCtx)
				stopCancel()

				cancel()

				select {
				case err := <-startDone:
					if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, scheduler.ErrClosed) {
						t.Fatalf("iter %d start error: %v", iter, err)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("iter %d: Start did not unblock after stop/cancel", iter)
				}

				// Final Close must be idempotent and return ErrClosed or nil
				closeErr := s.Close()
				if closeErr != nil && !errors.Is(closeErr, scheduler.ErrClosed) {
					t.Fatalf("iter %d close error: %v", iter, closeErr)
				}
			}
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

			err := s.Register("long_task", "@every 1s", func(ctx context.Context) error {
				closeOnce(taskStarted)
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
			closeOnce(taskRelease)

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

// TestSchedulerConcurrentStartStopCloseRace runs concurrent Start, Stop, and Close calls
// across multiple goroutines to verify race freedom and deadlock resilience.
func TestSchedulerConcurrentStartStopCloseRace(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			_ = s.Register("tick", "@every 1s", func(context.Context) error {
				return nil
			})

			ctx := t.Context()

			var wg sync.WaitGroup

			// Goroutines calling Start
			for range 3 {
				wg.Go(func() {
					_ = s.Start(ctx)
				})
			}

			// Goroutines calling Stop with varied timeouts
			for i := range 6 {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					timeout := time.Duration(id*10+10) * time.Millisecond
					stopCtx, stopCancel := context.WithTimeout(context.Background(), timeout)
					defer stopCancel()
					_ = s.Stop(stopCtx)
				}(i)
			}

			// Goroutines calling Close
			for range 3 {
				wg.Go(func() {
					_ = s.Close()
				})
			}

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// all goroutines finished cleanly
			case <-time.After(8 * time.Second):
				t.Fatal("concurrent start/stop/close race deadlocked!")
			}
		})
	}
}
