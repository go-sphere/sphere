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

func TestManagerStopTaskReturnsStartError(t *testing.T) {
	manager := NewManager()
	expectedStartErr := errors.New("start failed during stop")
	worker := scripttask.NewScriptTask("worker", func(ctx context.Context) error {
		<-ctx.Done()
		return expectedStartErr
	}, nil)

	if err := manager.StartTask(context.Background(), "worker", worker); err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")

	stopErr := manager.StopTask(context.Background(), "worker")
	if !errors.Is(stopErr, expectedStartErr) {
		t.Fatalf("expected start error %v, got %v", expectedStartErr, stopErr)
	}

	waitErr := manager.Wait()
	if !errors.Is(waitErr, expectedStartErr) {
		t.Fatalf("expected wait to include start error, got %v", waitErr)
	}
}

func TestManagerStopTaskCallerTimeout(t *testing.T) {
	manager := NewManager(WithManagerCleanupTimeout(120 * time.Millisecond))
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

func TestManagerWaitAllowsConcurrentStopAll(t *testing.T) {
	manager := NewManager()
	worker := scripttask.NewScriptTask("worker", nil, nil)

	if err := manager.StartTask(context.Background(), "worker", worker); err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")

	waitErrCh := make(chan error, 1)
	go func() {
		waitErrCh <- manager.Wait()
	}()

	select {
	case err := <-waitErrCh:
		t.Fatalf("wait returned before stop all: %v", err)
	case <-time.After(40 * time.Millisecond):
	}

	stopErrCh := make(chan error, 1)
	go func() {
		stopErrCh <- manager.StopAll(context.Background())
	}()

	if err := waitError(t, stopErrCh, "stop all while wait is pending"); err != nil {
		t.Fatalf("expected stop all to succeed, got %v", err)
	}
	if err := waitError(t, waitErrCh, "manager wait after stop all"); err != nil {
		t.Fatalf("expected wait to succeed after stop all, got %v", err)
	}
}

func TestManagerStopAllCallerTimeout(t *testing.T) {
	manager := NewManager(WithManagerCleanupTimeout(120 * time.Millisecond))
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

func TestManagerCleanupTimeoutOption(t *testing.T) {
	manager := NewManager(WithManagerCleanupTimeout(40 * time.Millisecond))
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
		t.Fatalf("expected cleanup timeout, got %v", stopErr)
	}
	if elapsed := time.Since(begin); elapsed < 40*time.Millisecond {
		t.Fatalf("expected stop to wait for cleanup timeout, elapsed=%s", elapsed)
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

// TestManagerStopTaskReturnsTombstoneResult covers BUG-49: after a task exits on
// its own and is removed, StopTask must surface the cached result rather than
// ErrTaskNotFound.
func TestManagerStopTaskReturnsTombstoneResult(t *testing.T) {
	manager := NewManager()
	expectedErr := errors.New("worker start failed")
	failing := scripttask.NewScriptTask("failing", func(context.Context) error {
		return expectedErr
	}, nil)

	if err := manager.StartTask(context.Background(), "failing", failing); err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	// Task exits on its own and is removed by the run goroutine.
	if err := manager.Wait(); !errors.Is(err, expectedErr) {
		t.Fatalf("expected wait to include %v, got %v", expectedErr, err)
	}
	if manager.IsRunning("failing") {
		t.Fatal("expected failing task to be removed")
	}

	stopErr := manager.StopTask(context.Background(), "failing")
	if errors.Is(stopErr, ErrTaskNotFound) {
		t.Fatalf("expected cached tombstone result, got ErrTaskNotFound")
	}
	if !errors.Is(stopErr, expectedErr) {
		t.Fatalf("expected cached error %v, got %v", expectedErr, stopErr)
	}

	// GetTaskResult must expose the same cached result.
	ok, result := manager.GetTaskResult("failing")
	if !ok {
		t.Fatal("expected tombstone result to be reported")
	}
	if !errors.Is(result, expectedErr) {
		t.Fatalf("expected tombstone result %v, got %v", expectedErr, result)
	}
}

// TestManagerTombstoneClearedOnRestart covers the BUG-49 cleanup strategy:
// re-registering the same name clears the tombstone so stale results do not leak
// into the restarted task, and unknown names still report ErrTaskNotFound.
func TestManagerTombstoneClearedOnRestart(t *testing.T) {
	manager := NewManager()

	if ok, _ := manager.GetTaskResult("svc"); ok {
		t.Fatal("expected unknown name to be absent")
	}

	expectedErr := errors.New("boom")
	failing := scripttask.NewScriptTask("svc", func(context.Context) error {
		return expectedErr
	}, nil)
	if err := manager.StartTask(context.Background(), "svc", failing); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := manager.Wait(); !errors.Is(err, expectedErr) {
		t.Fatalf("expected wait to include %v, got %v", expectedErr, err)
	}
	if ok, result := manager.GetTaskResult("svc"); !ok || !errors.Is(result, expectedErr) {
		t.Fatalf("expected tombstone result, got result=%v ok=%v", result, ok)
	}

	// Re-registering the same name clears the tombstone.
	worker := scripttask.NewScriptTask("svc", nil, nil)
	if err := manager.StartTask(context.Background(), "svc", worker); err != nil {
		t.Fatalf("restart failed: %v", err)
	}
	waitSignalManager(t, worker.Started(), "worker started")

	ok, result := manager.GetTaskResult("svc")
	if !ok {
		t.Fatal("expected running task to be reported")
	}
	if result != nil {
		t.Fatalf("expected cleared tombstone (nil result), got %v", result)
	}

	if err := manager.StopTask(context.Background(), "svc"); err != nil {
		t.Fatalf("stop failed: %v", err)
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

// ctxCaptureTask hands its start context to the test and then returns on its
// own, so nobody ever calls Stop on it.
type ctxCaptureTask struct {
	identifier string
	ctxCh      chan context.Context
}

func (t *ctxCaptureTask) Identifier() string { return t.identifier }

func (t *ctxCaptureTask) Start(ctx context.Context) error {
	t.ctxCh <- ctx
	return nil
}

func (t *ctxCaptureTask) Stop(context.Context) error { return nil }

// TestSelfExitedTaskReleasesRunContext pins that a task exiting on its own gets
// its run context cancelled. That context is a child of the caller's ctx, and
// requestStop — the only other place the cancel func is invoked — is never
// reached on this path, so leaving it uncancelled pins the child to a long-lived
// parent for the rest of the process.
func TestSelfExitedTaskReleasesRunContext(t *testing.T) {
	manager := NewManager()
	parent := t.Context()

	task := &ctxCaptureTask{identifier: "selfexit", ctxCh: make(chan context.Context, 1)}
	if err := manager.StartTask(parent, "selfexit", task); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	// Wait blocks until the run goroutine has fully unwound, deferred cancel
	// included.
	if err := manager.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	runCtx := <-task.ctxCh
	if runCtx.Err() == nil {
		t.Error("run context was never cancelled; it stays registered on the parent ctx")
	}
	if parent.Err() != nil {
		t.Error("the caller's context must not have been cancelled")
	}
}
