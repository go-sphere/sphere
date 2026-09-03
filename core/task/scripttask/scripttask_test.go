package scripttask

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestScriptTask_Identifier(t *testing.T) {
	task := NewScriptTask("task-123", nil, nil)
	if task.Identifier() != "task-123" {
		t.Fatalf("expected 'task-123', got %q", task.Identifier())
	}
}

func TestScriptTask_StartStopLifecycle_CustomCallbacks(t *testing.T) {
	var startCalls, stopCalls atomic.Int32

	task := NewScriptTask(
		"worker",
		func(ctx context.Context) error {
			startCalls.Add(1)
			<-ctx.Done()
			return ctx.Err()
		},
		func(ctx context.Context) error {
			stopCalls.Add(1)
			return nil
		},
	)

	if task.IsStarted() || task.IsStopped() {
		t.Fatal("task should not be started or stopped before invocation")
	}

	ctx, cancel := context.WithCancel(context.Background())
	startDone := make(chan error, 1)
	go func() {
		startDone <- task.Start(ctx)
	}()

	select {
	case <-task.Started():
	case <-time.After(time.Second):
		t.Fatal("task.Started() channel did not close")
	}

	if !task.IsStarted() {
		t.Fatal("task.IsStarted() should be true")
	}

	// Call Stop while Start is running
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("task.Stop failed: %v", err)
	}

	select {
	case <-task.Stopped():
	case <-time.After(time.Second):
		t.Fatal("task.Stopped() channel did not close")
	}

	if !task.IsStopped() {
		t.Fatal("task.IsStopped() should be true")
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

func TestScriptTask_Start_NilOnStart_BlocksUntilContextCanceled(t *testing.T) {
	task := NewScriptTask("nil-start", nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	startDone := make(chan error, 1)
	go func() {
		startDone <- task.Start(ctx)
	}()

	select {
	case <-task.Started():
	case <-time.After(time.Second):
		t.Fatal("task.Started() channel did not close")
	}

	if !task.IsStarted() {
		t.Fatal("IsStarted should be true")
	}

	cancel()

	select {
	case err := <-startDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancel")
	}
}

func TestScriptTask_Start_ReturnsError(t *testing.T) {
	expectedErr := errors.New("start failed")
	task := NewScriptTask("failing-start", func(ctx context.Context) error {
		return expectedErr
	}, nil)

	err := task.Start(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if !task.IsStarted() {
		t.Fatal("IsStarted should be true even on failure")
	}
}

func TestScriptTask_Stop_NilOnStop(t *testing.T) {
	task := NewScriptTask("nil-stop", nil, nil)
	err := task.Stop(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on nil onStop, got %v", err)
	}
	if !task.IsStopped() {
		t.Fatal("IsStopped should be true")
	}
	select {
	case <-task.Stopped():
	default:
		t.Fatal("Stopped() channel should be closed")
	}
}

func TestScriptTask_Stop_ReturnsError(t *testing.T) {
	expectedErr := errors.New("stop failed")
	task := NewScriptTask("failing-stop", nil, func(ctx context.Context) error {
		return expectedErr
	})

	err := task.Stop(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if !task.IsStopped() {
		t.Fatal("IsStopped should be true")
	}
}

func TestScriptTask_Stop_MultipleCalls(t *testing.T) {
	var stopCount atomic.Int32
	task := NewScriptTask("multi-stop", nil, func(ctx context.Context) error {
		stopCount.Add(1)
		return nil
	})

	for i := range 3 {
		if err := task.Stop(context.Background()); err != nil {
			t.Fatalf("iteration %d: unexpected stop error: %v", i, err)
		}
	}

	if stopCount.Load() != 3 {
		t.Fatalf("expected onStop to be called 3 times, got %d", stopCount.Load())
	}
}
