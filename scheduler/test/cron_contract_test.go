package test

import (
	"context"
	"errors"
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
