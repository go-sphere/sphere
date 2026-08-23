package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task/scripttask"
)

// TestGroupStopBeforeStart pins the contract that a Stop arriving before Start
// is recorded rather than dropped: Stop is the only way to shut a group down and
// stopReqCh does not exist until Start creates it, so an ignored early Stop
// would leave the group running with no way to stop it. Members that never
// started receive neither Start nor Stop.
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

	if err := waitError(t, startErrCh, "group start result"); err != nil {
		t.Fatalf("expected nil start result after pending stop, got %v", err)
	}
	if worker.IsStarted() {
		t.Fatal("pending stop must not start members")
	}
	if worker.IsStopped() {
		t.Fatal("pending stop must not stop members that never started")
	}
	if !group.IsStopped() {
		t.Fatal("expected group to be stopped after honouring the pending stop")
	}
}

func TestGroupStartOnCanceledContextDoesNotStartMembers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := scripttask.NewScriptTask("worker", nil, nil)
	if err := NewGroup(worker).Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if worker.IsStarted() || worker.IsStopped() {
		t.Fatal("canceled parent must not start members")
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
	if !strings.Contains(err.Error(), "blocking-stop") {
		t.Fatalf("cleanup timeout should name the stuck task, got %v", err)
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
	if startErr == nil {
		t.Fatal("expected cleanup timeout once the Stop ctx also bounds member Stop")
	}
	if !strings.Contains(startErr.Error(), "still stopping") {
		t.Fatalf("start result = %v, want cleanup timeout naming the worker", startErr)
	}
	waitSignal(t, worker.Stopped(), "worker stopped")
}

func TestGroupStopContextBoundsMemberStop(t *testing.T) {
	worker := scripttask.NewScriptTask("worker", nil, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
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
	elapsed := time.Since(begin)
	if elapsed > 200*time.Millisecond {
		t.Fatalf("member Stop should honour caller deadline, elapsed=%s", elapsed)
	}
	if err == nil {
		t.Fatal("expected a stop/start error from the cancelled cleanup ctx")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
	if err := waitError(t, startErrCh, "group start after bounded stop"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("start result = %v, want DeadlineExceeded", err)
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
	// The infra stage must leave Start for the http stage to begin; a nil
	// onStart blocks until cancellation and would never let stage 1 start.
	db := scripttask.NewScriptTask("db", func(context.Context) error { return nil }, infraStop("db"))
	cache := scripttask.NewScriptTask("cache", func(context.Context) error { return nil }, infraStop("cache"))
	httpSrv := scripttask.NewScriptTask("http", func(ctx context.Context) error {
		// Block until the group is tearing down, like a real server.
		<-ctx.Done()
		return ctx.Err()
	}, func(context.Context) error {
		close(httpDrained)
		return nil
	})

	// wave 0: infra (db, cache); wave 1: http server. Start order is wave 0 then
	// wave 1; stop order must be http first (last wave), then db and cache.
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

// TestStagedGroupStartsWavesInOrder pins that a later stage only starts after
// the previous stage has fully finished Start: a stage that depends on its
// predecessor's resources must never race ahead of them.
func TestStagedGroupStartsWavesInOrder(t *testing.T) {
	enterStageA := make(chan struct{})
	releaseStageA := make(chan struct{})
	stageA := scripttask.NewScriptTask("stage-a", func(context.Context) error {
		close(enterStageA)
		<-releaseStageA
		return nil
	}, nil)
	stageBStarted := make(chan struct{}, 1)
	stageB := scripttask.NewScriptTask("stage-b", func(context.Context) error {
		stageBStarted <- struct{}{}
		return nil
	}, nil)

	group := NewStagedGroup([]Task{stageA}, []Task{stageB})

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, enterStageA, "stage-a start entered")

	// stageA is still starting; stageB must not have begun.
	select {
	case <-stageBStarted:
		t.Fatal("stage-b started before stage-a fully started")
	case <-time.After(80 * time.Millisecond):
	}

	close(releaseStageA)

	select {
	case <-stageBStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for stage-b start")
	}

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if err := waitError(t, startErrCh, "staged group result"); err != nil {
		t.Fatalf("expected nil start result, got %v", err)
	}
}

// TestStagedGroupAbortBetweenStagesDoesNotStartLaterStage pins the pre-stage
// abort: a Stop or parent cancel that is already pending when a later stage
// would begin must not launch that stage, and a stage that never started must
// not receive Stop. An empty first stage is required so the previous stage has
// no remaining-loop select to consume the request itself; that path is the
// working one and would hide a break-from-select that never leaves the stage
// loop. TestGroupStopBeforeStart stays the single-stage contract.
func TestStagedGroupAbortBetweenStagesDoesNotStartLaterStage(t *testing.T) {
	cases := []struct {
		name string
		ctx  func(t *testing.T, group *Group) context.Context
	}{
		{
			name: "pending stop",
			ctx: func(t *testing.T, group *Group) context.Context {
				t.Helper()
				if err := group.Stop(context.Background()); err != nil {
					t.Fatalf("stop before start: %v", err)
				}
				return context.Background()
			},
		},
		{
			name: "cancelled parent",
			ctx: func(t *testing.T, _ *Group) context.Context {
				t.Helper()
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			http := scripttask.NewScriptTask("http", nil, nil)
			group := NewStagedGroup([]Task{}, []Task{http})
			ctx := tc.ctx(t, group)

			startErrCh := make(chan error, 1)
			go func() {
				startErrCh <- group.Start(ctx)
			}()
			if err := waitError(t, startErrCh, "group start result"); err != nil {
				t.Fatalf("expected nil start result after abort, got %v", err)
			}

			if http.IsStarted() {
				t.Fatal("stage 1 must not start after a pending abort")
			}
			if http.IsStopped() {
				t.Fatal("stage 1 must not be stopped when it never started")
			}
			if !group.IsStopped() {
				t.Fatal("expected group to be stopped after honouring the abort")
			}
		})
	}
}

// TestStagedGroupPendingStopStartsNoStage is the non-empty counterpart of the
// empty-first-stage abort: no stage is launched, matching "stages whose start
// never began receive neither Start nor Stop".
func TestStagedGroupPendingStopStartsNoStage(t *testing.T) {
	infra := scripttask.NewScriptTask("infra", func(context.Context) error {
		return nil
	}, nil)
	http := scripttask.NewScriptTask("http", nil, nil)
	group := NewStagedGroup([]Task{infra}, []Task{http})

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("stop before start: %v", err)
	}

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()
	if err := waitError(t, startErrCh, "group start result"); err != nil {
		t.Fatalf("expected nil start result after pending stop, got %v", err)
	}

	if infra.IsStarted() || infra.IsStopped() {
		t.Fatal("stage 0 must not start or stop after a pending stop")
	}
	if http.IsStarted() || http.IsStopped() {
		t.Fatal("stage 1 must not start or stop after a pending stop")
	}
}

// TestStagedGroupStartFailureStopsStartedStages pins that a stage failing to
// start stops every already-started stage in reverse order, and that stages
// after the failure never start and never stop.
func TestStagedGroupStartFailureStopsStartedStages(t *testing.T) {
	expectedErr := errors.New("stage-b failed")
	failing := scripttask.NewScriptTask("stage-b-failing", func(context.Context) error {
		return expectedErr
	}, nil)
	infra := scripttask.NewScriptTask("stage-a-infra", func(context.Context) error {
		return nil
	}, func(context.Context) error {
		// The failing stage must fully drain before the stage it depends on
		// starts stopping.
		select {
		case <-failing.Stopped():
		default:
			t.Error("stage-a stopped before stage-b drained")
		}
		return nil
	})
	never := scripttask.NewScriptTask("stage-c-never", func(context.Context) error {
		return nil
	}, nil)

	group := NewStagedGroup([]Task{infra}, []Task{failing}, []Task{never})

	err := group.Start(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected start error %v, got %v", expectedErr, err)
	}

	waitSignal(t, infra.Stopped(), "stage-a stopped")
	waitSignal(t, failing.Stopped(), "stage-b stopped")
	if never.IsStarted() {
		t.Fatal("stage-c must not start after stage-b failed")
	}
	if never.IsStopped() {
		t.Fatal("stage-c must not stop when never started")
	}
}

// stagingDB is a database task whose Stop closes it; writes after the close are
// refused, modelling the use-after-close the cascade reproduction observed.
type stagingDB struct {
	closed   atomic.Bool
	writeCnt atomic.Int64
}

func (d *stagingDB) Identifier() string { return "stage-db" }

func (d *stagingDB) Start(context.Context) error { return nil }

func (d *stagingDB) Stop(context.Context) error {
	d.closed.Store(true)
	return nil
}

func (d *stagingDB) write() error {
	if d.closed.Load() {
		return errors.New("db write after close")
	}
	d.writeCnt.Add(1)
	return nil
}

// stagingConsumer models an MQ consumer: Start returns immediately, a handler
// goroutine writes to the database, and Stop drains that handler before
// returning, exactly like the real mq PubSub closer.
type stagingConsumer struct {
	db          *stagingDB
	started     chan struct{}
	trigger     chan struct{}
	handlerDone chan error
}

func (c *stagingConsumer) Identifier() string { return "stage-mq" }

func (c *stagingConsumer) Start(ctx context.Context) error {
	close(c.started)
	go func() {
		<-c.trigger
		c.handlerDone <- c.db.write()
	}()
	// A running task's lifetime is its Start call: like the real consumer it
	// stays in Start until the group tears down, which is what keeps the
	// group's own Start alive.
	<-ctx.Done()
	return ctx.Err()
}

func (c *stagingConsumer) Stop(context.Context) error {
	close(c.trigger)
	// Drain: the handler must finish before this stop returns, otherwise the
	// preceding stage could close the database underneath it.
	return <-c.handlerDone
}

// TestStagedGroupDrainsConsumersBeforeClosingDependencies pins the cascade
// guarantee: on staged shutdown the handler wave (last stage) fully drains
// before the dependency wave (first stage) is torn down, so a handler
// executing during shutdown never writes a closed database.
func TestStagedGroupDrainsConsumersBeforeClosingDependencies(t *testing.T) {
	dbTask := &stagingDB{}
	mqTask := &stagingConsumer{
		db:          dbTask,
		started:     make(chan struct{}),
		trigger:     make(chan struct{}),
		handlerDone: make(chan error, 1),
	}

	group := NewStagedGroup([]Task{dbTask}, []Task{mqTask})

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	waitSignal(t, mqTask.started, "consumer started")

	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("staged stop failed: %v", err)
	}
	if err := waitError(t, startErrCh, "staged group result"); err != nil {
		t.Fatalf("expected nil start result, got %v", err)
	}

	if writes := dbTask.writeCnt.Load(); writes != 1 {
		t.Fatalf("handler writes = %d, want 1", writes)
	}
	if err := dbTask.write(); err == nil {
		t.Fatal("db must be closed after stop")
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
