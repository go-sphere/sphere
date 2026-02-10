package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task/scripttask"
)

func TestManagerStartAndStopTask(t *testing.T) {
	manager := NewManager()
	worker := scripttask.NewScriptTask("worker", nil, nil)

	if err := manager.StartTask(context.Background(), "worker", worker); err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")

	if !manager.IsRunning("worker") {
		t.Fatal("expected worker to be running")
	}

	if err := manager.StopTask(context.Background(), "worker"); err != nil {
		t.Fatalf("stop task failed: %v", err)
	}
	waitSignalManager(t, worker.Stopped(), "worker stopped")

	if manager.IsRunning("worker") {
		t.Fatal("expected worker to be removed after stop")
	}
	if err := manager.Wait(); err != nil {
		t.Fatalf("expected wait to succeed, got %v", err)
	}
}

func TestManagerStartTaskAlreadyExists(t *testing.T) {
	manager := NewManager()
	first := scripttask.NewScriptTask("first", nil, nil)
	second := scripttask.NewScriptTask("second", nil, nil)

	if err := manager.StartTask(context.Background(), "same", first); err != nil {
		t.Fatalf("start first task failed: %v", err)
	}
	waitSignalManager(t, first.Started(), "first started")

	if err := manager.StartTask(context.Background(), "same", second); !errors.Is(err, ErrTaskAlreadyExists) {
		t.Fatalf("expected ErrTaskAlreadyExists, got %v", err)
	}

	if err := manager.StopTask(context.Background(), "same"); err != nil {
		t.Fatalf("stop task failed: %v", err)
	}
}

func TestManagerStopTaskNotFound(t *testing.T) {
	manager := NewManager()

	if err := manager.StopTask(context.Background(), "missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestManagerWaitIncludesStartError(t *testing.T) {
	manager := NewManager()
	expectedErr := errors.New("start failed")
	failing := scripttask.NewScriptTask("failing", func(context.Context) error {
		return expectedErr
	}, nil)

	if err := manager.StartTask(context.Background(), "failing", failing); err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	waitErr := manager.Wait()
	if !errors.Is(waitErr, expectedErr) {
		t.Fatalf("expected wait to include %v, got %v", expectedErr, waitErr)
	}
	if manager.IsRunning("failing") {
		t.Fatal("expected failing task to be removed")
	}
}

func TestManagerStopTaskReturnsStopError(t *testing.T) {
	manager := NewManager()
	expectedStopErr := errors.New("stop failed")
	worker := scripttask.NewScriptTask("worker", nil, func(context.Context) error {
		return expectedStopErr
	})

	if err := manager.StartTask(context.Background(), "worker", worker); err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")

	stopErr := manager.StopTask(context.Background(), "worker")
	if !errors.Is(stopErr, expectedStopErr) {
		t.Fatalf("expected stop error %v, got %v", expectedStopErr, stopErr)
	}

	waitErr := manager.Wait()
	if !errors.Is(waitErr, expectedStopErr) {
		t.Fatalf("expected wait to include stop error, got %v", waitErr)
	}
}

func TestManagerStopTaskCallerTimeout(t *testing.T) {
	manager := NewManager(WithManagerAutoStopTimeout(120 * time.Millisecond))
	worker := scripttask.NewScriptTask("worker", nil, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	if err := manager.StartTask(context.Background(), "worker", worker); err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")

	stopCtx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	begin := time.Now()
	stopErr := manager.StopTask(stopCtx, "worker")
	if !errors.Is(stopErr, context.DeadlineExceeded) {
		t.Fatalf("expected stop timeout, got %v", stopErr)
	}
	if elapsed := time.Since(begin); elapsed >= 120*time.Millisecond {
		t.Fatalf("expected caller timeout to return early, elapsed=%s", elapsed)
	}
}

func TestManagerStopAllConcurrent(t *testing.T) {
	manager := NewManager()
	stopDelay := 150 * time.Millisecond

	workerA := scripttask.NewScriptTask("a", nil, func(context.Context) error {
		time.Sleep(stopDelay)
		return nil
	})
	workerB := scripttask.NewScriptTask("b", nil, func(context.Context) error {
		time.Sleep(stopDelay)
		return nil
	})
	workerC := scripttask.NewScriptTask("c", nil, func(context.Context) error {
		time.Sleep(stopDelay)
		return nil
	})

	if err := manager.StartTask(context.Background(), "a", workerA); err != nil {
		t.Fatalf("start a failed: %v", err)
	}
	if err := manager.StartTask(context.Background(), "b", workerB); err != nil {
		t.Fatalf("start b failed: %v", err)
	}
	if err := manager.StartTask(context.Background(), "c", workerC); err != nil {
		t.Fatalf("start c failed: %v", err)
	}
	waitSignalManager(t, workerA.Started(), "a started")
	waitSignalManager(t, workerB.Started(), "b started")
	waitSignalManager(t, workerC.Started(), "c started")

	begin := time.Now()
	if err := manager.StopAll(context.Background()); err != nil {
		t.Fatalf("stop all failed: %v", err)
	}
	elapsed := time.Since(begin)
	if elapsed >= 320*time.Millisecond {
		t.Fatalf("expected concurrent stop all around %s, got %s", stopDelay, elapsed)
	}

	if count := manager.GetTaskCount(); count != 0 {
		t.Fatalf("expected no running tasks, got %d", count)
	}
	if err := manager.Wait(); err != nil {
		t.Fatalf("expected wait success, got %v", err)
	}
}

func TestManagerStopAllCallerTimeout(t *testing.T) {
	manager := NewManager(WithManagerAutoStopTimeout(120 * time.Millisecond))
	worker := scripttask.NewScriptTask("worker", nil, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	if err := manager.StartTask(context.Background(), "worker", worker); err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")

	stopCtx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	stopErr := manager.StopAll(stopCtx)
	if !errors.Is(stopErr, context.DeadlineExceeded) {
		t.Fatalf("expected stop-all timeout, got %v", stopErr)
	}
}

func TestManagerAutoStopTimeoutOption(t *testing.T) {
	manager := NewManager(WithManagerAutoStopTimeout(40 * time.Millisecond))
	worker := scripttask.NewScriptTask("worker", nil, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	if err := manager.StartTask(context.Background(), "worker", worker); err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")

	begin := time.Now()
	stopErr := manager.StopTask(context.Background(), "worker")
	if !errors.Is(stopErr, context.DeadlineExceeded) {
		t.Fatalf("expected auto-stop timeout, got %v", stopErr)
	}
	if elapsed := time.Since(begin); elapsed < 40*time.Millisecond {
		t.Fatalf("expected stop to wait for auto-timeout, elapsed=%s", elapsed)
	}
}

func TestManagerCanRestartNameAfterTaskExit(t *testing.T) {
	manager := NewManager()

	oneShot := scripttask.NewScriptTask("oneshot", func(context.Context) error {
		return nil
	}, nil)
	if err := manager.StartTask(context.Background(), "service", oneShot); err != nil {
		t.Fatalf("start oneshot failed: %v", err)
	}
	if err := manager.Wait(); err != nil {
		t.Fatalf("expected first wait success, got %v", err)
	}

	worker := scripttask.NewScriptTask("worker", nil, nil)
	if err := manager.StartTask(context.Background(), "service", worker); err != nil {
		t.Fatalf("expected to reuse task name after exit, got %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")
	if err := manager.StopTask(context.Background(), "service"); err != nil {
		t.Fatalf("stop worker failed: %v", err)
	}
}

func waitSignalManager(t *testing.T, ch <-chan struct{}, desc string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for %s", desc)
	}
}
