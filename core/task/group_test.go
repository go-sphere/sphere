package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task/scripttask"
)

// TestGroupStopBeforeStart pins the contract that a Stop arriving before Start
// is recorded rather than dropped: Stop is the only way to shut a group down and
// stopReqCh does not exist until Start creates it, so an ignored early Stop
// would leave the group running with no way to stop it.
func TestGroupStopBeforeStart(t *testing.T) {
	worker := scripttask.NewScriptTask("worker", nil, nil)
	group := NewGroup(worker)

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("expected stop before start to succeed, got %v", err)
	}

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	// The pending stop must tear the group down without any further Stop call.
	if err := waitError(t, startErrCh, "group start result"); err != nil {
		t.Fatalf("expected nil start result after pending stop, got %v", err)
	}
	waitSignal(t, worker.Stopped(), "worker stopped")

	if !group.IsStopped() {
		t.Fatal("expected group to be stopped after honouring the pending stop")
	}
}

func TestGroupStopAfterStart(t *testing.T) {
	worker := scripttask.NewScriptTask("worker", nil, nil)
	group := NewGroup(worker)

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

func TestGroupCleanupTimeoutOption(t *testing.T) {
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
		WithCleanupTimeout(40*time.Millisecond),
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

func TestGroupManualStopIncludesTaskError(t *testing.T) {
	expectedErr := errors.New("worker failed during manual stop")
	releaseStart := make(chan struct{})
	var releaseOnce sync.Once
	worker := scripttask.NewScriptTask("worker", func(context.Context) error {
		<-releaseStart
		return expectedErr
	}, func(context.Context) error {
		releaseOnce.Do(func() {
			close(releaseStart)
		})
		return nil
	})

	group := NewGroup(worker)
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, worker.Started(), "worker started")

	if err := group.Stop(context.Background()); !errors.Is(err, expectedErr) {
		t.Fatalf("expected manual stop to include task error %v, got %v", expectedErr, err)
	}
	if err := waitError(t, startErrCh, "group start after manual stop"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected start result to include task error %v, got %v", expectedErr, err)
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

func TestGroupParentCancelIncludesTaskError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	expectedErr := errors.New("worker failed during parent cancel")
	releaseStart := make(chan struct{})
	var releaseOnce sync.Once
	worker := scripttask.NewScriptTask("worker", func(context.Context) error {
		<-releaseStart
		return expectedErr
	}, func(context.Context) error {
		releaseOnce.Do(func() {
			close(releaseStart)
		})
		return nil
	})
	group := NewGroup(worker)

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(ctx)
	}()

	waitSignal(t, worker.Started(), "worker started")
	cancel()

	if err := waitError(t, startErrCh, "group start after parent cancel"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected parent cancel to include task error %v, got %v", expectedErr, err)
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

// TestGroupWrappedCanceledCountsAsFailure covers BUG-35: a task that fails on
// its own with an error wrapping context.Canceled (rather than being cancelled
// by the group) must count as a failure, propagate, and stop the other tasks.
func TestGroupWrappedCanceledCountsAsFailure(t *testing.T) {
	businessErr := fmt.Errorf("business failure: %w", context.Canceled)
	failing := scripttask.NewScriptTask("failing", func(context.Context) error {
		return businessErr
	}, nil)
	worker := scripttask.NewScriptTask("worker", nil, nil)

	group := NewGroup(worker, failing)

	err := group.Start(context.Background())
	if !errors.Is(err, businessErr) {
		t.Fatalf("expected wrapped-canceled business error in result, got %v", err)
	}

	waitSignal(t, worker.Stopped(), "worker stopped")
	waitSignal(t, failing.Stopped(), "failing stopped")
}

// TestStagedGroupStopsInReverseWaveOrder covers ENC-15: staged groups stop the
// last wave first and only begin the previous wave once it has fully drained.
func TestStagedGroupStopsInReverseWaveOrder(t *testing.T) {
	httpDrained := make(chan struct{})
	httpSrv := scripttask.NewScriptTask("http", nil, func(context.Context) error {
		close(httpDrained)
		return nil
	})

	var mu sync.Mutex
	var stopOrder []string
	infraStop := func(id string) func(context.Context) error {
		return func(context.Context) error {
			// The infra wave must not start stopping until the http wave drained.
			select {
			case <-httpDrained:
			default:
				t.Errorf("%s stopped before http wave drained", id)
			}
			mu.Lock()
			stopOrder = append(stopOrder, id)
			mu.Unlock()
			return nil
		}
	}
	db := scripttask.NewScriptTask("db", nil, infraStop("db"))
	cache := scripttask.NewScriptTask("cache", nil, infraStop("cache"))

	// wave 0: infra (db, cache); wave 1: http server. Stop order must be http
	// first (last wave), then db and cache.
	group := NewStagedGroup(
		[]Task{db, cache},
		[]Task{httpSrv},
	)

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, db.Started(), "db started")
	waitSignal(t, cache.Started(), "cache started")
	waitSignal(t, httpSrv.Started(), "http started")

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("staged stop failed: %v", err)
	}
	if err := waitError(t, startErrCh, "staged group result"); err != nil {
		t.Fatalf("expected nil start result, got %v", err)
	}

	waitSignal(t, httpSrv.Stopped(), "http stopped")

	mu.Lock()
	defer mu.Unlock()
	if len(stopOrder) != 2 {
		t.Fatalf("expected both infra tasks to stop, got %v", stopOrder)
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
