package task_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

func TestFunc_Identifier(t *testing.T) {
	tk := task.NewFunc("task-123", nil, nil)
	if tk.Identifier() != "task-123" {
		t.Fatalf("expected %q, got %q", "task-123", tk.Identifier())
	}
}

func TestFunc_StartStopLifecycle_CustomCallbacks(t *testing.T) {
	var startCalls, stopCalls atomic.Int32
	tk := task.NewFunc(
		"worker",
		func(ctx context.Context) error {
			startCalls.Add(1)
			<-ctx.Done()
			return ctx.Err()
		},
		func(context.Context) error {
			stopCalls.Add(1)
			return nil
		},
	)

	ctx, cancel := context.WithCancel(t.Context())
	startDone := make(chan error, 1)
	go func() {
		startDone <- tk.Start(ctx)
	}()

	if err := tk.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	cancel()

	select {
	case err := <-startDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancellation")
	}

	if startCalls.Load() != 1 {
		t.Fatalf("expected 1 start call, got %d", startCalls.Load())
	}
	if stopCalls.Load() != 1 {
		t.Fatalf("expected 1 stop call, got %d", stopCalls.Load())
	}
}

func TestFunc_Start_NilOnStart(t *testing.T) {
	tk := task.NewFunc("nil-start", nil, nil)
	if err := tk.Start(t.Context()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestFunc_Start_ReturnsError(t *testing.T) {
	want := errors.New("start failed")
	tk := task.NewFunc("failing-start", func(context.Context) error {
		return want
	}, nil)

	err := tk.Start(t.Context())
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestFunc_Stop_NilOnStop(t *testing.T) {
	tk := task.NewFunc("nil-stop", nil, nil)
	if err := tk.Stop(t.Context()); err != nil {
		t.Fatalf("expected nil error on nil onStop, got %v", err)
	}
}

func TestFunc_Stop_ReturnsError(t *testing.T) {
	want := errors.New("stop failed")
	tk := task.NewFunc("failing-stop", nil, func(context.Context) error {
		return want
	})

	err := tk.Stop(t.Context())
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestFunc_Stop_MultipleCalls(t *testing.T) {
	var stopCount atomic.Int32
	tk := task.NewFunc("multi-stop", nil, func(context.Context) error {
		stopCount.Add(1)
		return nil
	})

	for i := range 3 {
		if err := tk.Stop(t.Context()); err != nil {
			t.Fatalf("iteration %d: unexpected stop error: %v", i, err)
		}
	}

	if stopCount.Load() != 3 {
		t.Fatalf("expected onStop to be called 3 times, got %d", stopCount.Load())
	}
}

func TestFunc_LifecycleContract(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		return task.NewFunc("contract", nil, nil)
	})
}
