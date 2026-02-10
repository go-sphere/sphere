package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task/scripttask"
)

func TestGroupStopBeforeStart(t *testing.T) {
	worker := scripttask.NewScriptTask("worker", nil, nil)
	group := NewGroup(worker)

	if err := group.Stop(context.Background()); !errors.Is(err, ErrGroupNotStarted) {
		t.Fatalf("expected ErrGroupNotStarted, got %v", err)
	}

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, worker.Started(), "worker started")

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("expected stop to succeed after start, got %v", err)
	}

	if err := waitError(t, startErrCh, "group start result"); err != nil {
		t.Fatalf("expected nil start result after graceful stop, got %v", err)
	}
}

func TestGroupTaskErrorFailFast(t *testing.T) {
	expectedErr := errors.New("start failed")
	failing := scripttask.NewScriptTask("failing", func(context.Context) error {
		return expectedErr
	}, nil)
	workerA := scripttask.NewScriptTask("worker-a", nil, nil)
	workerB := scripttask.NewScriptTask("worker-b", nil, nil)

	group := NewGroup(workerA, workerB, failing)

	err := group.Start(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected start error %v, got %v", expectedErr, err)
	}

	waitSignal(t, workerA.Stopped(), "worker-a stopped")
	waitSignal(t, workerB.Stopped(), "worker-b stopped")
	waitSignal(t, failing.Stopped(), "failing stopped")
}

func TestGroupAutoStopTimeoutOption(t *testing.T) {
	expectedErr := errors.New("boom")
	failing := scripttask.NewScriptTask("failing", func(context.Context) error {
		return expectedErr
	}, nil)
	blockingStop := scripttask.NewScriptTask("blocking-stop", nil, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	group := NewGroupWithOptions(
		[]Task{blockingStop, failing},
		WithAutoStopTimeout(40*time.Millisecond),
	)

	err := group.Start(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected start error %v, got %v", expectedErr, err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected auto stop timeout in result, got %v", err)
	}
}

func TestGroupTaskPanicFailFast(t *testing.T) {
	panicTask := scripttask.NewScriptTask("panic", func(context.Context) error {
		panic("panic for test")
	}, nil)
	worker := scripttask.NewScriptTask("worker", nil, nil)

	group := NewGroup(worker, panicTask)
	err := group.Start(context.Background())
	if err == nil {
		t.Fatal("expected panic-derived error, got nil")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Fatalf("expected panic details in error, got %v", err)
	}

	waitSignal(t, worker.Stopped(), "worker stopped")
	waitSignal(t, panicTask.Stopped(), "panic task stopped")
}

func TestGroupManualStopGraceful(t *testing.T) {
	releaseStop := make(chan struct{})
	worker := scripttask.NewScriptTask("worker", nil, func(context.Context) error {
		<-releaseStop
		return nil
	})

	group := NewGroup(worker)
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, worker.Started(), "worker started")

	stopErrCh := make(chan error, 1)
	go func() {
		stopErrCh <- group.Stop(context.Background())
	}()

	select {
	case err := <-stopErrCh:
		t.Fatalf("stop returned before cleanup release: %v", err)
	case <-time.After(80 * time.Millisecond):
	}

	close(releaseStop)

	if err := waitError(t, stopErrCh, "manual stop result"); err != nil {
		t.Fatalf("expected manual stop success, got %v", err)
	}
	if err := waitError(t, startErrCh, "group start after manual stop"); err != nil {
		t.Fatalf("expected start to return nil after manual stop, got %v", err)
	}
}

func TestGroupParentCancelGraceful(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	worker := scripttask.NewScriptTask("worker", nil, nil)
	group := NewGroup(worker)

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(ctx)
	}()

	waitSignal(t, worker.Started(), "worker started")
	cancel()

	if err := waitError(t, startErrCh, "group start after parent cancel"); err != nil {
		t.Fatalf("expected nil after parent cancel, got %v", err)
	}
	waitSignal(t, worker.Stopped(), "worker stopped")
}

func TestGroupStartNilContinue(t *testing.T) {
	oneShot := scripttask.NewScriptTask("oneshot", func(context.Context) error {
		return nil
	}, nil)
	worker := scripttask.NewScriptTask("worker", nil, nil)

	group := NewGroup(oneShot, worker)
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, oneShot.Started(), "oneshot started")
	waitSignal(t, worker.Started(), "worker started")

	select {
	case err := <-startErrCh:
		t.Fatalf("group exited early after Start(nil): %v", err)
	case <-time.After(80 * time.Millisecond):
	}

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("manual stop failed: %v", err)
	}
	if err := waitError(t, startErrCh, "group result after stop"); err != nil {
		t.Fatalf("expected nil start result after stop, got %v", err)
	}
	waitSignal(t, oneShot.Stopped(), "oneshot stopped")
	waitSignal(t, worker.Stopped(), "worker stopped")
}

func TestGroupSingleUse(t *testing.T) {
	worker := scripttask.NewScriptTask("worker", nil, nil)
	group := NewGroup(worker)

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, worker.Started(), "worker started")

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if err := waitError(t, startErrCh, "first start result"); err != nil {
		t.Fatalf("expected nil first start result, got %v", err)
	}

	if err := group.Start(context.Background()); !errors.Is(err, ErrGroupAlreadyStopped) {
		t.Fatalf("expected ErrGroupAlreadyStopped, got %v", err)
	}
}

func TestGroupStartTwiceWhileRunning(t *testing.T) {
	worker := scripttask.NewScriptTask("worker", nil, nil)
	group := NewGroup(worker)

	firstStartErr := make(chan error, 1)
	go func() {
		firstStartErr <- group.Start(context.Background())
	}()

	waitSignal(t, worker.Started(), "worker started")

	if err := group.Start(context.Background()); !errors.Is(err, ErrGroupAlreadyStarted) {
		t.Fatalf("expected ErrGroupAlreadyStarted, got %v", err)
	}

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if err := waitError(t, firstStartErr, "first start result"); err != nil {
		t.Fatalf("expected nil first start result, got %v", err)
	}
}

func TestGroupStopContextTimeout(t *testing.T) {
	stopDelay := 180 * time.Millisecond
	worker := scripttask.NewScriptTask("worker", nil, func(context.Context) error {
		time.Sleep(stopDelay)
		return nil
	})
	group := NewGroup(worker)

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, worker.Started(), "worker started")

	stopCtx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	begin := time.Now()
	err := group.Stop(stopCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected stop timeout, got %v", err)
	}
	if elapsed := time.Since(begin); elapsed > stopDelay {
		t.Fatalf("stop should return on caller timeout, elapsed=%s", elapsed)
	}

	startErr := waitError(t, startErrCh, "group result after timed stop")
	if startErr != nil {
		t.Fatalf("expected start result nil after background cleanup, got %v", startErr)
	}
}

func TestGroupConcurrentStop(t *testing.T) {
	stopDelay := 150 * time.Millisecond
	workerA := scripttask.NewScriptTask("worker-a", nil, func(context.Context) error {
		time.Sleep(stopDelay)
		return nil
	})
	workerB := scripttask.NewScriptTask("worker-b", nil, func(context.Context) error {
		time.Sleep(stopDelay)
		return nil
	})
	workerC := scripttask.NewScriptTask("worker-c", nil, func(context.Context) error {
		time.Sleep(stopDelay)
		return nil
	})

	group := NewGroup(workerA, workerB, workerC)
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, workerA.Started(), "worker-a started")
	waitSignal(t, workerB.Started(), "worker-b started")
	waitSignal(t, workerC.Started(), "worker-c started")

	begin := time.Now()
	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	elapsed := time.Since(begin)

	if elapsed >= 320*time.Millisecond {
		t.Fatalf("expected concurrent stop (~%s), got %s", stopDelay, elapsed)
	}

	if err := waitError(t, startErrCh, "start result after concurrent stop"); err != nil {
		t.Fatalf("expected nil start result, got %v", err)
	}
}

func waitSignal(t *testing.T, ch <-chan struct{}, desc string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for %s", desc)
	}
}

func waitError(t *testing.T, ch <-chan error, desc string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for %s", desc)
		return nil
	}
}
