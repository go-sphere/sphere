package boot_test

import (
	"context"
	"errors"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/core/task"
)

type bootHungTask struct {
	id          string
	stopCalled  atomic.Bool
	stopStarted atomic.Bool
	startReady  chan struct{}
	stopDone    chan struct{}
}

func newBootHungTask(id string) *bootHungTask {
	return &bootHungTask{
		id:         id,
		startReady: make(chan struct{}),
		stopDone:   make(chan struct{}),
	}
}

func (h *bootHungTask) Identifier() string {
	return h.id
}

func (h *bootHungTask) Start(ctx context.Context) error {
	close(h.startReady)
	<-ctx.Done()
	return ctx.Err()
}

func (h *bootHungTask) Stop(ctx context.Context) error {
	h.stopCalled.Store(true)
	h.stopStarted.Store(true)
	select {
	case <-h.stopDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestBootShutdownScenarios(t *testing.T) {
	t.Run("ShutdownTimeoutUnblocksHangingStop", func(t *testing.T) {
		tk := newBootHungTask("hung-stop-task")
		shutdownTimeout := 60 * time.Millisecond

		type config struct{}
		conf := &config{}

		var afterStopExecuted atomic.Bool
		var afterStopCtxLive atomic.Bool

		runDone := make(chan error, 1)
		go func() {
			runDone <- boot.Run(
				conf,
				func(_ *config) (*boot.Application, error) {
					return boot.NewApplication(tk), nil
				},
				boot.WithShutdownTimeout(shutdownTimeout),
				boot.WithShutdownSignals(syscall.SIGUSR1),
				boot.AddAfterStop(func(ctx context.Context) error {
					afterStopExecuted.Store(true)
					if ctx.Err() == nil {
						afterStopCtxLive.Store(true)
					}
					return nil
				}),
			)
		}()

		select {
		case <-tk.startReady:
		case <-time.After(time.Second):
			t.Fatal("task did not start")
		}

		// Trigger shutdown via signal
		startShutdown := time.Now()
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR1); err != nil {
			t.Fatalf("send shutdown signal: %v", err)
		}

		select {
		case err := <-runDone:
			elapsed := time.Since(startShutdown)
			if elapsed > 1*time.Second {
				t.Fatalf("shutdown took too long (%v), bound not respected", elapsed)
			}
			if !afterStopExecuted.Load() {
				t.Fatalf("expected afterStop hook to execute")
			}
			if !afterStopCtxLive.Load() {
				t.Fatalf("expected afterStop hook to receive live fallback context")
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("run error = %v, want context.DeadlineExceeded", err)
			}
			if !tk.stopCalled.Load() {
				t.Fatal("task Stop was not called")
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("boot.Run deadlocked on hanging Stop")
		}
	})

	t.Run("PanickingHooksAcrossLifecycle", func(t *testing.T) {
		type config struct{}
		conf := &config{}

		t.Run("BeforeStartPanic", func(t *testing.T) {
			err := boot.Run(
				conf,
				func(_ *config) (*boot.Application, error) {
					return boot.NewApplication(newBootHungTask("dummy")), nil
				},
				boot.AddBeforeStart(func(ctx context.Context) error {
					panic("panic in beforeStart")
				}),
			)
			if err == nil {
				t.Fatal("expected error on beforeStart panic, got nil")
			}
		})

		t.Run("BeforeStopPanic", func(t *testing.T) {
			oneShot := task.NewGroup() // completes immediately
			var afterRan atomic.Bool
			err := boot.Run(
				conf,
				func(_ *config) (*boot.Application, error) {
					return boot.NewApplication(oneShot), nil
				},
				boot.AddBeforeStop(func(ctx context.Context) error {
					panic("panic in beforeStop")
				}),
				boot.AddAfterStop(func(ctx context.Context) error {
					afterRan.Store(true)
					return nil
				}),
			)
			if err == nil {
				t.Fatal("expected error on beforeStop panic, got nil")
			}
			if !afterRan.Load() {
				t.Fatal("expected afterStop to run even when beforeStop panics")
			}
		})

		t.Run("AfterStopPanic", func(t *testing.T) {
			oneShot := task.NewGroup()
			err := boot.Run(
				conf,
				func(_ *config) (*boot.Application, error) {
					return boot.NewApplication(oneShot), nil
				},
				boot.AddAfterStop(func(ctx context.Context) error {
					panic("panic in afterStop")
				}),
			)
			if err == nil {
				t.Fatal("expected error on afterStop panic, got nil")
			}
		})
	})
}
