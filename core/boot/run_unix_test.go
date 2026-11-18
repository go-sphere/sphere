//go:build unix

package boot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

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

	if !task.startCalled.Load() {
		t.Error("Task Start was not called")
	}
	if !task.stopCalled.Load() {
		t.Error("Task Stop was not called")
	}
}

func TestRun_EmptyShutdownSignalsDoesNotShutdownOnSIGWINCH(t *testing.T) {
	task := &mockTask{identifier: "test-task"}
	opts := newOptions(
		WithShutdownTimeout(time.Second),
		WithShutdownSignals(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, task, opts)
	}()
	time.Sleep(50 * time.Millisecond)

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}
	if err := proc.Signal(syscall.SIGWINCH); err != nil {
		t.Fatalf("signal: %v", err)
	}

	select {
	case err := <-done:
		t.Fatalf("empty WithShutdownSignals shut down on SIGWINCH: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ctx cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for ctx cancel")
	}
}

func TestRun_ShutdownTimeoutBoundsGroupedTaskStop(t *testing.T) {
	task := &mockTask{
		identifier: "slow-stop",
		startFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		stopFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	opts := newOptions(WithShutdownTimeout(100 * time.Millisecond))

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), NewApplication(task), opts)
	}()
	time.Sleep(50 * time.Millisecond)
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	start := time.Now()
	err := <-done
	if duration := time.Since(start); duration > 500*time.Millisecond {
		t.Fatalf("grouped Stop should honour shutdown timeout, took %v", duration)
	}
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
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

	if !task.stopCalled.Load() {
		t.Error("Task Stop was not called")
	}
}

func TestRun_ConcurrentSignals(t *testing.T) {
	started := make(chan struct{})
	var stopOnce sync.Once
	stopEntered := make(chan struct{})
	stopRelease := make(chan struct{})
	task := &mockTask{
		identifier: "test-task",
		startFunc: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return nil
		},
		stopFunc: func(context.Context) error {
			stopOnce.Do(func() { close(stopEntered) })
			<-stopRelease
			return nil
		},
	}
	opts := newOptions(
		WithShutdownTimeout(1*time.Second),
		WithShutdownSignals(syscall.SIGUSR1, syscall.SIGWINCH),
	)

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), task, opts)
	}()

	<-started

	// Send multiple non-terminating signals so the test process cannot die on a
	// second signal arriving after run deregisters signal handlers.
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGUSR1)
	<-stopEntered
	_ = proc.Signal(syscall.SIGWINCH)
	_ = proc.Signal(syscall.SIGUSR1)
	close(stopRelease)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out")
	}

	if !task.stopCalled.Load() {
		t.Error("Task Stop was not called")
	}
}

func TestRun_Integration(t *testing.T) {
	// Simulates a realistic scenario with all components
	var (
		logMu       sync.Mutex
		logMessages []string
	)
	addLogMessage := func(message string) {
		logMu.Lock()
		defer logMu.Unlock()
		logMessages = append(logMessages, message)
	}

	task := &mockTask{
		identifier: "integration-task",
		startFunc: func(ctx context.Context) error {
			addLogMessage("task started")
			<-ctx.Done()
			return nil
		},
		stopFunc: func(ctx context.Context) error {
			addLogMessage("task stopped")
			return nil
		},
	}

	opts := newOptions(
		WithShutdownTimeout(2*time.Second),
		AddBeforeStart(func(ctx context.Context) error {
			addLogMessage("initializing resources")
			return nil
		}),
		AddBeforeStop(func(ctx context.Context) error {
			addLogMessage("preparing shutdown")
			return nil
		}),
		AddAfterStop(func(ctx context.Context) error {
			// Verify context is still valid for cleanup
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled too early in afterStop")
			default:
				addLogMessage("cleanup complete")
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
