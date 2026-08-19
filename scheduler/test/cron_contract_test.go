package test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/scheduler"
)

func TestCronContractRegisterTriggers(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			var count atomic.Int32
			if err := s.Register("tick", "@every 1s", func(context.Context) error {
				count.Add(1)
				return nil
			}); err != nil {
				t.Fatalf("register: %v", err)
			}
			startCronRuntime(t, s)
			waitFor(t, 3*time.Second, func() bool { return count.Load() >= 1 })
		})
	}
}

func TestCronContractDuplicateName(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			handler := func(context.Context) error { return nil }
			if err := s.Register("dup", "@every 1s", handler); err != nil {
				t.Fatalf("register first: %v", err)
			}
			if err := s.Register("dup", "@every 1s", handler); !errors.Is(err, scheduler.ErrDuplicateName) {
				t.Fatalf("register duplicate error = %v, want %v", err, scheduler.ErrDuplicateName)
			}
		})
	}
}

// TestCronContractReRegisterAfterUnregister covers reusing a name after it was
// unregistered. The asynq driver mounts handlers on a ServeMux that panics on a
// duplicate pattern and cannot detach an entry, so re-registering must reuse the
// existing mux entry and rebind the handler rather than mounting it twice.
func TestCronContractReRegisterAfterUnregister(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)

			if err := s.Register("recycled", "@every 1s", func(context.Context) error {
				return nil
			}); err != nil {
				t.Fatalf("register: %v", err)
			}
			if err := s.Unregister("recycled"); err != nil {
				t.Fatalf("unregister: %v", err)
			}

			var count atomic.Int32
			if err := s.Register("recycled", "@every 1s", func(context.Context) error {
				count.Add(1)
				return nil
			}); err != nil {
				t.Fatalf("re-register after unregister: %v", err)
			}

			// The rebound handler must actually receive triggers, not just register.
			startCronRuntime(t, s)
			waitFor(t, 5*time.Second, func() bool { return count.Load() >= 1 })
		})
	}
}

func TestCronContractUnregisterStopsTriggers(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			var count atomic.Int32
			if err := s.Register("tick", "@every 1s", func(context.Context) error {
				count.Add(1)
				return nil
			}); err != nil {
				t.Fatalf("register: %v", err)
			}
			startCronRuntime(t, s)
			waitFor(t, 3*time.Second, func() bool { return count.Load() >= 1 })
			if err := s.Unregister("tick"); err != nil {
				t.Fatalf("unregister: %v", err)
			}
			got := count.Load()
			time.Sleep(1200 * time.Millisecond)
			if count.Load() != got {
				t.Fatalf("handler triggered after unregister: before=%d after=%d", got, count.Load())
			}
		})
	}
}

func TestCronContractStopWaitsForRunningHandler(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			started := make(chan struct{})
			release := make(chan struct{})
			if err := s.Register("blocking", "@every 1s", func(context.Context) error {
				closeOnce(started)
				<-release
				return nil
			}); err != nil {
				t.Fatalf("register: %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			doneStart := make(chan error, 1)
			go func() { doneStart <- s.Start(ctx) }()
			waitForChan(t, 3*time.Second, started)

			stopDone := make(chan error, 1)
			go func() {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer stopCancel()
				stopDone <- s.Stop(stopCtx)
			}()

			select {
			case err := <-stopDone:
				t.Fatalf("stop returned before handler finished: %v", err)
			case <-time.After(150 * time.Millisecond):
			}
			close(release)
			if err := <-stopDone; err != nil {
				t.Fatalf("stop: %v", err)
			}
			cancel()
			err := <-doneStart
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("start: %v", err)
			}
		})
	}
}

// TestCronContractStopKeepsHandlerContextLiveWhileDraining pins that a graceful
// Stop is a real drain: an in-flight handler must keep a usable context while
// Stop waits for it. Cancelling the run context up front would hand every
// handler a dead context and reduce the wait to a formality.
func TestCronContractStopKeepsHandlerContextLiveWhileDraining(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			started := make(chan struct{})
			release := make(chan struct{})
			handlerCtx := make(chan context.Context, 1)

			if err := s.Register("blocking", "@every 1s", func(ctx context.Context) error {
				select {
				case handlerCtx <- ctx:
				default:
				}
				closeOnce(started)
				<-release
				return nil
			}); err != nil {
				t.Fatalf("register: %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			doneStart := make(chan error, 1)
			go func() { doneStart <- s.Start(ctx) }()
			waitForChan(t, 3*time.Second, started)
			running := <-handlerCtx

			stopDone := make(chan error, 1)
			go func() {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer stopCancel()
				stopDone <- s.Stop(stopCtx)
			}()

			// Let Stop enter its drain, then check the handler is still usable.
			time.Sleep(200 * time.Millisecond)
			if err := running.Err(); err != nil {
				t.Errorf("in-flight handler context was cancelled while Stop was still draining: %v", err)
			}

			close(release)
			if err := <-stopDone; err != nil {
				t.Fatalf("stop: %v", err)
			}
			cancel()
			if err := <-doneStart; err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("start: %v", err)
			}
		})
	}
}

// TestCronContractStopRacingStartFullyShutsDown covers the window between Start
// taking ownership of the lifecycle and the underlying runtime actually coming
// up. A Stop landing inside that window must not report the scheduler closed
// while leaving the runtime live — a leaked runtime keeps firing handlers and,
// for the asynq driver, keeps using a Redis client the caller is free to close.
//
// The window is narrow, so this is a stress test: it repeats the race and checks
// that handlers have genuinely stopped once everything has settled.
func TestCronContractStopRacingStartFullyShutsDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skip scheduler start/stop race stress test in short mode")
	}
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			for attempt := range 5 {
				s := factory.new(t)
				var count atomic.Int32
				if err := s.Register("tick", "@every 1s", func(context.Context) error {
					count.Add(1)
					return nil
				}); err != nil {
					t.Fatalf("attempt %d register: %v", attempt, err)
				}

				ctx, cancel := context.WithCancel(context.Background())
				startDone := make(chan error, 1)
				go func() { startDone <- s.Start(ctx) }()

				// Pile several Stops onto the startup sequence to widen the race.
				var stoppers sync.WaitGroup
				for range 8 {
					stoppers.Go(func() { _ = s.Stop(context.Background()) })
				}
				stoppers.Wait()

				// Whatever the interleaving was, a final Stop must leave the
				// scheduler quiesced.
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = s.Stop(stopCtx)
				stopCancel()
				cancel()
				select {
				case err := <-startDone:
					if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, scheduler.ErrClosed) {
						t.Fatalf("attempt %d start: %v", attempt, err)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("attempt %d: Start did not return", attempt)
				}

				settled := count.Load()
				time.Sleep(1200 * time.Millisecond)
				if got := count.Load(); got != settled {
					t.Fatalf("attempt %d: handler still firing after shutdown: %d -> %d", attempt, settled, got)
				}
			}
		})
	}
}

func TestCronContractStopTimeoutThenRetrySucceeds(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			started := make(chan struct{})
			release := make(chan struct{})
			if err := s.Register("blocking", "@every 1s", func(context.Context) error {
				closeOnce(started)
				<-release
				return nil
			}); err != nil {
				t.Fatalf("register: %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			doneStart := make(chan error, 1)
			go func() { doneStart <- s.Start(ctx) }()
			waitForChan(t, 3*time.Second, started)

			// First Stop times out while the handler is still blocked.
			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer timeoutCancel()
			if err := s.Stop(timeoutCtx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("first stop error = %v, want %v", err, context.DeadlineExceeded)
			}

			// Let the handler finish, then retry Stop with a fresh context: it must
			// wait on the same shutdown signal and complete successfully.
			close(release)
			retryCtx, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer retryCancel()
			if err := s.Stop(retryCtx); err != nil {
				t.Fatalf("second stop: %v", err)
			}

			// State must now be closed, not stuck in stopping.
			if err := s.Register("after", "@every 1s", func(context.Context) error { return nil }); !errors.Is(err, scheduler.ErrClosed) {
				t.Fatalf("register after stop error = %v, want %v", err, scheduler.ErrClosed)
			}
			if err := s.Close(); !errors.Is(err, scheduler.ErrClosed) {
				t.Fatalf("close after stop error = %v, want %v", err, scheduler.ErrClosed)
			}

			cancel()
			err := <-doneStart
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("start: %v", err)
			}
		})
	}
}

func TestCronContractCloseThenMethodsReturnClosed(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new(t)
			if err := s.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
			if err := s.Register("closed", "@every 1s", func(context.Context) error { return nil }); !errors.Is(err, scheduler.ErrClosed) {
				t.Fatalf("register after close error = %v, want %v", err, scheduler.ErrClosed)
			}
			if err := s.Unregister("closed"); !errors.Is(err, scheduler.ErrClosed) {
				t.Fatalf("unregister after close error = %v, want %v", err, scheduler.ErrClosed)
			}
		})
	}
}

func waitFor(tb testing.TB, timeout time.Duration, ok func() bool) {
	tb.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	tb.Fatalf("condition not met within %s", timeout)
}

func waitForChan(tb testing.TB, timeout time.Duration, ch <-chan struct{}) {
	tb.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		tb.Fatalf("channel not signaled within %s", timeout)
	}
}

func closeOnce(ch chan struct{}) {
	defer func() { _ = recover() }()
	close(ch)
}
