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
	"github.com/go-sphere/sphere/core/task/tasktest"
)

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

// TestStagedGroupStopBudgetExhaustedStillStopsEarlierStages pins that a stage
// whose Stop begins only after the shared cleanup budget is exhausted still
// receives a live context. Previously every wave shared one context with a
// single deadline: a slow last-stage drain that used up the whole budget made
// the earlier stage's ctx-honouring Stop fail immediately with
// DeadlineExceeded and skip the cleanup the staged order exists to protect.
func TestStagedGroupStopBudgetExhaustedStillStopsEarlierStages(t *testing.T) {
	var closerRan atomic.Bool
	closerCtxLive := make(chan bool, 1)
	closer := tasktest.NewFake("closer")
	closer.Mode = tasktest.ModeOneshot
	closer.StopFunc = func(ctx context.Context) error {
		closerCtxLive <- ctx.Err() == nil
		closerRan.Store(true)
		return nil
	}
	httpSrv := tasktest.NewFake("http")
	httpSrv.Mode = tasktest.ModeServer
	// The http Stop blocks until the shared cleanup budget expires, so the
	// closer wave only starts after the deadline has passed.
	httpSrv.StopFunc = func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	group := task.NewStagedGroupWithOptions(
		[][]task.Task{{closer}, {httpSrv}},
		task.WithCleanupTimeout(60*time.Millisecond),
	)
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- group.Start(context.Background())
	}()

	select {
	case <-httpSrv.Started():
	case <-time.After(2 * time.Second):
		t.Fatal("http did not start")
	}
	// A natural complete does not happen while the server stage blocks in
	// Start, so request shutdown manually, as boot.Run would. The http stage
	// exceeds the cleanup budget, so the stop result is expected to carry a
	// deadline error — what this test pins is that the earlier stage still
	// stops afterwards, under a live context.
	if stopErr := group.Stop(context.Background()); stopErr == nil {
		t.Fatal("expected a deadline error from the over-budget http stage, got nil")
	}

	select {
	case ctxLive := <-closerCtxLive:
		if !ctxLive {
			t.Fatal("earlier-stage Stop received an already-expired context")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closer Stop was never called after the budget was exhausted")
	}
	if !closerRan.Load() {
		t.Fatal("closer Stop did not run")
	}
	if err := waitFakeErr(t, startErrCh, "staged group result"); err == nil {
		t.Fatal("expected the stop deadline error, got nil")
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
