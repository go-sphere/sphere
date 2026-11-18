package boot_test

import (
	"context"
	"sync/atomic"
	"testing"

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
	runUnixShutdownScenarios(t)

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
