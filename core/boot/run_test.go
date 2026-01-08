package boot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
)

// mockTask implements task.Task for testing
type mockTask struct {
	identifier    string
	startFunc     func(ctx context.Context) error
	stopFunc      func(ctx context.Context) error
	startCalled   bool
	stopCalled    bool
	startDuration time.Duration
}

func (m *mockTask) Identifier() string {
	return m.identifier
}

func (m *mockTask) Start(ctx context.Context) error {
	m.startCalled = true
	if m.startDuration > 0 {
		select {
		case <-time.After(m.startDuration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if m.startFunc != nil {
		return m.startFunc(ctx)
	}
	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

func (m *mockTask) Stop(ctx context.Context) error {
	m.stopCalled = true
	if m.stopFunc != nil {
		return m.stopFunc(ctx)
	}
	return nil
}

func TestRun_NormalStartAndStop(t *testing.T) {
	task := &mockTask{identifier: "test-task"}
	opts := newOptions(WithShutdownTimeout(1 * time.Second))

	// Run task and send signal after a short delay
	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), task, opts)
	}()

	// Give task time to start
	time.Sleep(50 * time.Millisecond)

	// Send interrupt signal
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out")
	}

	if !task.startCalled {
		t.Error("Task Start was not called")
	}
	if !task.stopCalled {
		t.Error("Task Stop was not called")
	}
}

func TestRun_TaskStartError(t *testing.T) {
	expectedErr := errors.New("start failed")
	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			return expectedErr
		},
	}
	opts := newOptions(WithShutdownTimeout(1 * time.Second))

	err := run(context.Background(), task, opts)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap %v, got: %v", expectedErr, err)
	}

	if !task.startCalled {
		t.Error("Task Start was not called")
	}
	if !task.stopCalled {
		t.Error("Task Stop was not called")
	}
}

func TestRun_TaskStartPanic(t *testing.T) {
	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			panic("something went wrong")
		},
	}
	opts := newOptions(WithShutdownTimeout(1 * time.Second))

	err := run(context.Background(), task, opts)
	if err == nil {
		t.Fatal("Expected error from panic, got nil")
	}

	if !errors.Is(err, context.Canceled) && !task.stopCalled {
		t.Error("Task Stop should be called after panic")
	}
}

func TestRun_TaskStopError(t *testing.T) {
	stopErr := errors.New("stop failed")
	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			// Return immediately to trigger shutdown
			return nil
		},
		stopFunc: func(ctx context.Context) error {
			return stopErr
		},
	}
	opts := newOptions(WithShutdownTimeout(1 * time.Second))

	err := run(context.Background(), task, opts)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(err, stopErr) {
		t.Errorf("Expected error to contain stop error, got: %v", err)
	}
}

func TestRun_BeforeStartHookError(t *testing.T) {
	hookErr := errors.New("before start hook failed")
	task := &mockTask{identifier: "test-task"}
	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		AddBeforeStart(func(ctx context.Context) error {
			return hookErr
		}),
	)

	err := run(context.Background(), task, opts)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(err, hookErr) {
		t.Errorf("Expected error to wrap hook error, got: %v", err)
	}

	// Task should not start if before start hook fails
	if task.startCalled {
		t.Error("Task Start should not be called when beforeStart hook fails")
	}
}

func TestRun_BeforeStopHookError(t *testing.T) {
	hookErr := errors.New("before stop hook failed")
	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			// Return immediately to trigger shutdown
			return nil
		},
	}
	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		AddBeforeStop(func(ctx context.Context) error {
			return hookErr
		}),
	)

	err := run(context.Background(), task, opts)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(err, hookErr) {
		t.Errorf("Expected error to contain hook error, got: %v", err)
	}

	if !task.stopCalled {
		t.Error("Task Stop should still be called even if beforeStop hook fails")
	}
}

func TestRun_AfterStopHookError(t *testing.T) {
	hookErr := errors.New("after stop hook failed")
	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			// Return immediately to trigger shutdown
			return nil
		},
	}
	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		AddAfterStop(func(ctx context.Context) error {
			return hookErr
		}),
	)

	err := run(context.Background(), task, opts)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(err, hookErr) {
		t.Errorf("Expected error to contain hook error, got: %v", err)
	}
}

func TestRun_AfterStopHookContextNotCanceled(t *testing.T) {
	contextChecked := false
	contextValid := false

	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			// Return immediately to trigger shutdown
			return nil
		},
	}
	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		AddAfterStop(func(ctx context.Context) error {
			contextChecked = true
			// Check if context is still valid (not cancelled yet)
			select {
			case <-ctx.Done():
				// Context is already cancelled - this is the BUG we're testing for
				contextValid = false
			default:
				// Context is still valid - correct behavior
				contextValid = true
			}
			return nil
		}),
	)

	err := run(context.Background(), task, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !contextChecked {
		t.Fatal("afterStop hook was not executed")
	}

	if !contextValid {
		t.Error("Context passed to afterStop hook was already cancelled - this is the BUG!")
	}
}

func TestRun_MultipleErrors(t *testing.T) {
	startErr := errors.New("start error")
	stopErr := errors.New("stop error")
	hookErr := errors.New("hook error")

	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			return startErr
		},
		stopFunc: func(ctx context.Context) error {
			return stopErr
		},
	}
	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		AddBeforeStop(func(ctx context.Context) error {
			return hookErr
		}),
	)

	err := run(context.Background(), task, opts)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// All errors should be included
	if !errors.Is(err, startErr) {
		t.Errorf("Expected error to contain start error")
	}
	if !errors.Is(err, stopErr) {
		t.Errorf("Expected error to contain stop error")
	}
	if !errors.Is(err, hookErr) {
		t.Errorf("Expected error to contain hook error")
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	task := &mockTask{identifier: "test-task"}
	opts := newOptions(WithShutdownTimeout(1 * time.Second))

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, task, opts)
	}()

	// Give task time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out")
	}

	if !task.startCalled {
		t.Error("Task Start was not called")
	}
	if !task.stopCalled {
		t.Error("Task Stop was not called")
	}
}

func TestRun_ShutdownTimeout(t *testing.T) {
	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			// Return immediately to trigger shutdown
			return nil
		},
		stopFunc: func(ctx context.Context) error {
			// Simulate slow shutdown that exceeds timeout
			select {
			case <-time.After(2 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	opts := newOptions(WithShutdownTimeout(100 * time.Millisecond))

	start := time.Now()
	err := run(context.Background(), task, opts)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded error, got: %v", err)
	}

	// Should timeout around 100ms, not wait for 2 seconds
	if duration > 500*time.Millisecond {
		t.Errorf("Shutdown took too long: %v", duration)
	}
}

func TestRun_CustomSignals(t *testing.T) {
	task := &mockTask{identifier: "test-task"}
	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		WithShutdownSignals(syscall.SIGUSR1),
	)

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), task, opts)
	}()

	// Give task time to start
	time.Sleep(50 * time.Millisecond)

	// Send custom signal
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGUSR1)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out")
	}

	if !task.stopCalled {
		t.Error("Task Stop was not called")
	}
}

func TestRun_HooksExecutionOrder(t *testing.T) {
	var executionOrder []string

	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			executionOrder = append(executionOrder, "start")
			return nil
		},
		stopFunc: func(ctx context.Context) error {
			executionOrder = append(executionOrder, "stop")
			return nil
		},
	}

	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		AddBeforeStart(func(ctx context.Context) error {
			executionOrder = append(executionOrder, "beforeStart")
			return nil
		}),
		AddBeforeStop(func(ctx context.Context) error {
			executionOrder = append(executionOrder, "beforeStop")
			return nil
		}),
		AddAfterStop(func(ctx context.Context) error {
			executionOrder = append(executionOrder, "afterStop")
			return nil
		}),
	)

	err := run(context.Background(), task, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := []string{"beforeStart", "start", "beforeStop", "stop", "afterStop"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("Expected %d execution steps, got %d: %v", len(expected), len(executionOrder), executionOrder)
	}

	for i, step := range expected {
		if executionOrder[i] != step {
			t.Errorf("Step %d: expected %s, got %s", i, step, executionOrder[i])
		}
	}
}

func TestRun_MultipleHooks(t *testing.T) {
	var beforeStartCount, beforeStopCount, afterStopCount int

	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			return nil
		},
	}

	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		AddBeforeStart(func(ctx context.Context) error {
			beforeStartCount++
			return nil
		}),
		AddBeforeStart(func(ctx context.Context) error {
			beforeStartCount++
			return nil
		}),
		AddBeforeStop(func(ctx context.Context) error {
			beforeStopCount++
			return nil
		}),
		AddBeforeStop(func(ctx context.Context) error {
			beforeStopCount++
			return nil
		}),
		AddAfterStop(func(ctx context.Context) error {
			afterStopCount++
			return nil
		}),
		AddAfterStop(func(ctx context.Context) error {
			afterStopCount++
			return nil
		}),
	)

	err := run(context.Background(), task, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if beforeStartCount != 2 {
		t.Errorf("Expected 2 beforeStart hooks, got %d", beforeStartCount)
	}
	if beforeStopCount != 2 {
		t.Errorf("Expected 2 beforeStop hooks, got %d", beforeStopCount)
	}
	if afterStopCount != 2 {
		t.Errorf("Expected 2 afterStop hooks, got %d", afterStopCount)
	}
}

func TestRun_TaskCompletesNormally(t *testing.T) {
	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			// Task completes without error
			return nil
		},
	}
	opts := newOptions(WithShutdownTimeout(1 * time.Second))

	err := run(context.Background(), task, opts)
	if err != nil {
		t.Errorf("Expected no error when task completes normally, got: %v", err)
	}

	if !task.startCalled {
		t.Error("Task Start was not called")
	}
	if !task.stopCalled {
		t.Error("Task Stop was not called after normal completion")
	}
}

func TestRun_HookAccessesValidContext(t *testing.T) {
	tests := []struct {
		name     string
		hookType string
		addHook  func(Hook) Option
	}{
		{
			name:     "beforeStart hook",
			hookType: "beforeStart",
			addHook:  AddBeforeStart,
		},
		{
			name:     "beforeStop hook",
			hookType: "beforeStop",
			addHook:  AddBeforeStop,
		},
		{
			name:     "afterStop hook",
			hookType: "afterStop",
			addHook:  AddAfterStop,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextValid := false
			task := &mockTask{
				identifier: "test-task",
				startFunc: func(ctx context.Context) error {
					return nil
				},
			}

			opts := newOptions(
				WithShutdownTimeout(1*time.Second),
				tt.addHook(func(ctx context.Context) error {
					// Verify context has a deadline (for shutdown context)
					// or is not cancelled (for operation contexts)
					select {
					case <-ctx.Done():
						// Already cancelled
						contextValid = false
					default:
						contextValid = true
					}
					return nil
				}),
			)

			err := run(context.Background(), task, opts)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !contextValid {
				t.Errorf("%s received invalid/cancelled context", tt.hookType)
			}
		})
	}
}

func TestRun_ConcurrentSignals(t *testing.T) {
	task := &mockTask{identifier: "test-task"}
	opts := newOptions(WithShutdownTimeout(1 * time.Second))

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), task, opts)
	}()

	// Give task time to start
	time.Sleep(50 * time.Millisecond)

	// Send multiple signals
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)
	_ = proc.Signal(syscall.SIGTERM)
	_ = proc.Signal(syscall.SIGINT)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out")
	}

	if !task.stopCalled {
		t.Error("Task Stop was not called")
	}
}

func TestRun_Integration(t *testing.T) {
	// Simulates a realistic scenario with all components
	var logMessages []string

	task := &mockTask{
		identifier: "integration-task",
		startFunc: func(ctx context.Context) error {
			logMessages = append(logMessages, "task started")
			<-ctx.Done()
			return nil
		},
		stopFunc: func(ctx context.Context) error {
			logMessages = append(logMessages, "task stopped")
			return nil
		},
	}

	opts := newOptions(
		WithShutdownTimeout(2*time.Second),
		AddBeforeStart(func(ctx context.Context) error {
			logMessages = append(logMessages, "initializing resources")
			return nil
		}),
		AddBeforeStop(func(ctx context.Context) error {
			logMessages = append(logMessages, "preparing shutdown")
			return nil
		}),
		AddAfterStop(func(ctx context.Context) error {
			// Verify context is still valid for cleanup
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled too early in afterStop")
			default:
				logMessages = append(logMessages, "cleanup complete")
				return nil
			}
		}),
	)

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), task, opts)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}

	expectedMessages := []string{
		"initializing resources",
		"task started",
		"preparing shutdown",
		"task stopped",
		"cleanup complete",
	}

	if len(logMessages) != len(expectedMessages) {
		t.Fatalf("Expected %d log messages, got %d: %v", len(expectedMessages), len(logMessages), logMessages)
	}

	for i, expected := range expectedMessages {
		if logMessages[i] != expected {
			t.Errorf("Message %d: expected %q, got %q", i, expected, logMessages[i])
		}
	}
}
