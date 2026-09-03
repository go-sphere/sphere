package task_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/scripttask"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

type groupHangingTask struct {
	id          string
	ignoreCtx   bool
	startCalled atomic.Bool
	stopCalled  atomic.Bool
}

func (h *groupHangingTask) Identifier() string {
	return h.id
}

func (h *groupHangingTask) Start(ctx context.Context) error {
	h.startCalled.Store(true)
	if h.ignoreCtx {
		select {} // hangs forever
	}
	<-ctx.Done()
	return ctx.Err()
}

func (h *groupHangingTask) Stop(ctx context.Context) error {
	h.stopCalled.Store(true)
	if h.ignoreCtx {
		select {} // hangs forever
	}
	<-ctx.Done()
	return ctx.Err()
}

type groupRecordingTask struct {
	id          string
	startFn     func(ctx context.Context) error
	stopFn      func(ctx context.Context) error
	startCalled atomic.Bool
	stopCalled  atomic.Bool
}

func (r *groupRecordingTask) Identifier() string { return r.id }
func (r *groupRecordingTask) Start(ctx context.Context) error {
	r.startCalled.Store(true)
	if r.startFn != nil {
		return r.startFn(ctx)
	}
	<-ctx.Done()
	return ctx.Err()
}
func (r *groupRecordingTask) Stop(ctx context.Context) error {
	r.stopCalled.Store(true)
	if r.stopFn != nil {
		return r.stopFn(ctx)
	}
	return nil
}

// TestAdversarialGroup_MultiStage_Failures tests multi-stage staged groups with failures
// at stage 0, stage 1, or stage 2, verifying staged reverse stop order and that
// subsequent unstarted stages never receive Start or Stop.
func TestAdversarialGroup_MultiStage_Failures(t *testing.T) {
	t.Parallel()

	t.Run("Stage1Fails_StagedReverseStop_Stage2NeverStarts", func(t *testing.T) {
		t.Parallel()

		var stopOrderMu sync.Mutex
		var stopOrder []string

		t0_1 := &groupRecordingTask{id: "s0_t1", startFn: func(ctx context.Context) error { return nil }, stopFn: func(ctx context.Context) error {
			stopOrderMu.Lock()
			stopOrder = append(stopOrder, "s0_t1")
			stopOrderMu.Unlock()
			return nil
		}}
		t0_2 := &groupRecordingTask{id: "s0_t2", startFn: func(ctx context.Context) error { return nil }, stopFn: func(ctx context.Context) error {
			stopOrderMu.Lock()
			stopOrder = append(stopOrder, "s0_t2")
			stopOrderMu.Unlock()
			return nil
		}}

		errFail := errors.New("stage 1 boom")
		t1_1 := &groupRecordingTask{id: "s1_t1", startFn: func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return errFail
		}, stopFn: func(ctx context.Context) error {
			stopOrderMu.Lock()
			stopOrder = append(stopOrder, "s1_t1")
			stopOrderMu.Unlock()
			return nil
		}}
		t1_2 := &groupRecordingTask{id: "s1_t2", startFn: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}, stopFn: func(ctx context.Context) error {
			stopOrderMu.Lock()
			stopOrder = append(stopOrder, "s1_t2")
			stopOrderMu.Unlock()
			return nil
		}}

		t2_1 := &groupRecordingTask{id: "s2_t1"}
		t2_2 := &groupRecordingTask{id: "s2_t2"}

		g := task.NewStagedGroup(
			[]task.Task{t0_1, t0_2},
			[]task.Task{t1_1, t1_2},
			[]task.Task{t2_1, t2_2},
		)

		err := g.Start(context.Background())
		if !errors.Is(err, errFail) {
			t.Fatalf("expected error containing %v, got %v", errFail, err)
		}

		// Verify stage 0 and stage 1 were started and stopped
		if !t0_1.startCalled.Load() || !t0_1.stopCalled.Load() {
			t.Errorf("expected stage 0 task 1 started & stopped")
		}
		if !t0_2.startCalled.Load() || !t0_2.stopCalled.Load() {
			t.Errorf("expected stage 0 task 2 started & stopped")
		}
		if !t1_1.startCalled.Load() || !t1_1.stopCalled.Load() {
			t.Errorf("expected stage 1 task 1 started & stopped")
		}
		if !t1_2.startCalled.Load() || !t1_2.stopCalled.Load() {
			t.Errorf("expected stage 1 task 2 started & stopped")
		}

		// Verify stage 2 tasks were NEVER started and NEVER stopped
		if t2_1.startCalled.Load() || t2_1.stopCalled.Load() {
			t.Errorf("stage 2 task 1 should not have been touched")
		}
		if t2_2.startCalled.Load() || t2_2.stopCalled.Load() {
			t.Errorf("stage 2 task 2 should not have been touched")
		}

		// Verify stop order: stage 1 tasks must be stopped BEFORE stage 0 tasks
		stopOrderMu.Lock()
		defer stopOrderMu.Unlock()
		s1PosMax := -1
		s0PosMin := 999
		for pos, name := range stopOrder {
			if name == "s1_t1" || name == "s1_t2" {
				if pos > s1PosMax {
					s1PosMax = pos
				}
			}
			if name == "s0_t1" || name == "s0_t2" {
				if pos < s0PosMin {
					s0PosMin = pos
				}
			}
		}
		if s1PosMax > s0PosMin {
			t.Errorf("expected stage 1 tasks to stop before stage 0 tasks, got order: %v", stopOrder)
		}
	})

	t.Run("Stage0Fails_Stage1And2NeverTouch", func(t *testing.T) {
		t.Parallel()

		errFail := errors.New("s0 immediate fail")
		t0 := &groupRecordingTask{id: "s0", startFn: func(ctx context.Context) error { return errFail }}
		t1 := &groupRecordingTask{id: "s1"}
		t2 := &groupRecordingTask{id: "s2"}

		g := task.NewStagedGroup([]task.Task{t0}, []task.Task{t1}, []task.Task{t2})
		err := g.Start(context.Background())
		if !errors.Is(err, errFail) {
			t.Fatalf("expected %v, got %v", errFail, err)
		}

		if !t0.stopCalled.Load() {
			t.Errorf("expected s0 to be stopped")
		}
		if t1.startCalled.Load() || t1.stopCalled.Load() {
			t.Errorf("expected s1 untouched")
		}
		if t2.startCalled.Load() || t2.stopCalled.Load() {
			t.Errorf("expected s2 untouched")
		}
	})
}

// TestAdversarialGroup_HungStartTimeout tests that WithStartTimeout aborts stuck non-final stages
// and returns ErrStartTimeout.
func TestAdversarialGroup_HungStartTimeout(t *testing.T) {
	t.Parallel()

	stuckTask := &groupHangingTask{id: "stuck_in_start", ignoreCtx: false}
	finalTask := &groupRecordingTask{id: "final_task"}

	g := task.NewStagedGroupWithOptions(
		[][]task.Task{
			{stuckTask},
			{finalTask},
		},
		task.WithStartTimeout(50*time.Millisecond),
		task.WithCleanupTimeout(100*time.Millisecond),
	)

	start := time.Now()
	err := g.Start(context.Background())
	elapsed := time.Since(start)

	if !errors.Is(err, task.ErrStartTimeout) {
		t.Fatalf("expected ErrStartTimeout, got %v", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("start timeout took too long: %v", elapsed)
	}
	if finalTask.startCalled.Load() {
		t.Errorf("final stage should not have been started")
	}
}

// TestAdversarialGroup_HungStopCleanupTimeout tests that WithCleanupTimeout bounds Stop execution.
func TestAdversarialGroup_HungStopCleanupTimeout(t *testing.T) {
	t.Parallel()

	startEntered := make(chan struct{})
	slowStopTask := &groupRecordingTask{
		id: "slow_stop",
		startFn: func(ctx context.Context) error {
			close(startEntered)
			<-ctx.Done()
			return ctx.Err()
		},
		stopFn: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	g := task.NewGroupWithOptions(
		[]task.Task{slowStopTask},
		task.WithCleanupTimeout(60*time.Millisecond),
	)

	startDone := make(chan error, 1)
	go func() {
		startDone <- g.Start(t.Context())
	}()

	select {
	case <-startEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("task did not enter Start")
	}

	stopStart := time.Now()
	stopErr := g.Stop(t.Context())
	stopElapsed := time.Since(stopStart)

	if stopElapsed > 500*time.Millisecond {
		t.Errorf("stop took too long: %v", stopElapsed)
	}

	startErr := <-startDone
	if stopErr == nil && startErr == nil {
		t.Errorf("expected cleanup timeout error, got stopErr=%v startErr=%v", stopErr, startErr)
	}
}

// TestAdversarialGroup_ImmediateCancellationAndPreStop tests edge cases where context is canceled
// before Start or Stop is called before Start.
func TestAdversarialGroup_ImmediateCancellationAndPreStop(t *testing.T) {
	t.Parallel()

	t.Run("CanceledContextBeforeStart", func(t *testing.T) {
		t.Parallel()
		t1 := &groupRecordingTask{id: "t1"}
		g := task.NewGroup(t1)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := g.Start(ctx)
		if t1.startCalled.Load() && !t1.stopCalled.Load() {
			t.Errorf("task was started but never stopped")
		}
		_ = err
	})

	t.Run("StopBeforeStart", func(t *testing.T) {
		t.Parallel()
		t1 := &groupRecordingTask{id: "t1"}
		g := task.NewGroup(t1)

		err := g.Stop(context.Background())
		if err != nil {
			t.Errorf("unexpected error on pre-start Stop: %v", err)
		}

		err = g.Start(context.Background())
		if err != nil {
			t.Errorf("unexpected error on Start after pre-start Stop: %v", err)
		}
		if t1.startCalled.Load() {
			t.Errorf("task should not have started after pre-start Stop")
		}
	})
}

// TestAdversarialGroup_HighConcurrencyStress executes 50 concurrent Stop callers
// while tasks are rapidly starting, failing, or completing.
func TestAdversarialGroup_HighConcurrencyStress(t *testing.T) {
	t.Parallel()

	for iter := range 10 {
		tasks := make([]task.Task, 10)
		for i := range 10 {
			idx := i
			tasks[i] = scripttask.NewScriptTask(
				"stress_task",
				func(ctx context.Context) error {
					if idx%3 == 0 {
						time.Sleep(5 * time.Millisecond)
						return errors.New("err on start")
					}
					<-ctx.Done()
					return nil
				},
				func(ctx context.Context) error {
					return nil
				},
			)
		}

		g := task.NewGroupWithOptions(tasks, task.WithCleanupTimeout(50*time.Millisecond))

		startDone := make(chan error, 1)
		go func() {
			startDone <- g.Start(context.Background())
		}()

		var wg sync.WaitGroup
		const stoppers = 50
		wg.Add(stoppers)
		for range stoppers {
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				_ = g.Stop(ctx)
			}()
		}

		wg.Wait()
		select {
		case <-startDone:
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: Group.Start deadlocked", iter)
		}

		if !g.IsStopped() {
			t.Fatalf("iteration %d: Group should be in stopped state", iter)
		}
	}
}

// TestAdversarialGroup_NoBusySpin verifies that canceling a parent context does not cause
// a busy-spin (CPU loop) while remaining tasks take time to stop.
func TestAdversarialGroup_NoBusySpin(t *testing.T) {
	t.Parallel()

	slowTask := &groupRecordingTask{
		id: "slow",
		startFn: func(ctx context.Context) error {
			<-ctx.Done()
			time.Sleep(50 * time.Millisecond) // slow shutdown
			return nil
		},
	}

	g := task.NewGroup(slowTask)
	ctx, cancel := context.WithCancel(context.Background())

	startDone := make(chan error, 1)
	go func() {
		startDone <- g.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel() // trigger cancellation

	select {
	case <-startDone:
	case <-time.After(1 * time.Second):
		t.Fatal("Group did not exit within bounded time after context cancellation")
	}
}

// TestGroupNaturalCompleteStopsAllTasks pins that Stop is the cleanup half of
// the lifecycle even when every Start returns on its own. Previously the group
// marked itself stopped without calling Stop, and a later Group.Stop was a
// no-op, so one-shot tasks never released.
func TestGroupNaturalCompleteStopsAllTasks(t *testing.T) {
	a := tasktest.NewFake("a")
	a.Mode = tasktest.ModeOneshot
	b := tasktest.NewFake("b")
	b.Mode = tasktest.ModeOneshot

	if err := task.NewGroup(a, b).Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if a.StopCount() != 1 || b.StopCount() != 1 {
		t.Fatalf("Stop counts a=%d b=%d, want 1 each", a.StopCount(), b.StopCount())
	}
}

func TestGroupNaturalCompleteSurfacesStopError(t *testing.T) {
	flushErr := errors.New("flush failed")
	a := tasktest.NewFake("a")
	a.Mode = tasktest.ModeOneshot
	a.StopErr = flushErr

	err := task.NewGroup(a).Start(context.Background())
	if !errors.Is(err, flushErr) {
		t.Fatalf("start error = %v, want %v", err, flushErr)
	}
	if !strings.Contains(err.Error(), "a") {
		t.Fatalf("error %q should name the task", err)
	}
}

func TestStagedGroupNaturalCompleteStopsInReverseOrder(t *testing.T) {
	var mu sync.Mutex
	var stopOrder []string
	record := func(id string) func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			stopOrder = append(stopOrder, id)
			mu.Unlock()
			return nil
		}
	}

	db := tasktest.NewFake("db")
	db.Mode = tasktest.ModeOneshot
	db.StopFunc = record("db")
	mig := tasktest.NewFake("mig")
	mig.Mode = tasktest.ModeOneshot
	mig.StopFunc = record("mig")

	if err := task.NewStagedGroup([]task.Task{db}, []task.Task{mig}).Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(stopOrder) != 2 || stopOrder[0] != "mig" || stopOrder[1] != "db" {
		t.Fatalf("stop order %v, want [mig db]", stopOrder)
	}
}

func TestStagedGroupStartTimeoutAbortsNonFinalStage(t *testing.T) {
	stuck := tasktest.NewFake("stuck")
	stuck.Mode = tasktest.ModeServer
	later := tasktest.NewFake("later")
	later.Mode = tasktest.ModeOneshot

	group := task.NewStagedGroupWithOptions(
		[][]task.Task{{stuck}, {later}},
		task.WithStartTimeout(40*time.Millisecond),
	)
	err := group.Start(context.Background())
	if !errors.Is(err, task.ErrStartTimeout) {
		t.Fatalf("start error = %v, want ErrStartTimeout", err)
	}
	if !strings.Contains(err.Error(), "stuck") {
		t.Fatalf("timeout error %q should name the hung task", err)
	}
	if later.IsStarted() {
		t.Fatal("later stage must not start after a start timeout")
	}
	if stuck.StartCount() != 1 {
		t.Fatalf("stuck StartCount = %d, want 1", stuck.StartCount())
	}
	if stuck.StopCount() != 1 {
		t.Fatalf("timeout must Stop the hung stage, StopCount = %d", stuck.StopCount())
	}
}

func TestStagedGroupStartTimeoutDoesNotApplyToLastStage(t *testing.T) {
	infra := tasktest.NewFake("infra")
	infra.Mode = tasktest.ModeOneshot
	httpSrv := tasktest.NewFake("http")
	httpSrv.Mode = tasktest.ModeRunLoop

	group := task.NewStagedGroupWithOptions(
		[][]task.Task{{infra}, {httpSrv}},
		task.WithStartTimeout(40*time.Millisecond),
	)
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()
	waitFake(t, httpSrv.Started(), "http started")
	time.Sleep(80 * time.Millisecond)
	select {
	case err := <-startErrCh:
		t.Fatalf("last-stage run loop must not be start-timed-out: %v", err)
	default:
	}
	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := waitFakeErr(t, startErrCh, "group start result"); err != nil {
		t.Fatalf("expected nil start result, got %v", err)
	}
}

func TestGroupStartTimeoutDoesNotApplyToSingleStage(t *testing.T) {
	httpSrv := tasktest.NewFake("http")
	httpSrv.Mode = tasktest.ModeServer
	group := task.NewGroupWithOptions([]task.Task{httpSrv}, task.WithStartTimeout(40*time.Millisecond))

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()
	waitFake(t, httpSrv.Started(), "http started")
	time.Sleep(80 * time.Millisecond)
	select {
	case err := <-startErrCh:
		t.Fatalf("NewGroup must ignore WithStartTimeout, got %v", err)
	default:
	}
	if err := group.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := waitFakeErr(t, startErrCh, "group start result"); err != nil {
		t.Fatalf("expected nil start result, got %v", err)
	}
}

func waitFake(t *testing.T, ch <-chan struct{}, desc string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for %s", desc)
	}
}

func waitFakeErr(t *testing.T, ch <-chan error, desc string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for %s", desc)
		return nil
	}
}
