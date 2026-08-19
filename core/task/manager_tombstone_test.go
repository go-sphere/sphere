package task

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// stopErrTask returns an error from Stop only, after a delay, so that Start
// returns well before the stop result is available.
type stopErrTask struct {
	identifier string
	stopDelay  time.Duration
	stopErr    error
}

func (t *stopErrTask) Identifier() string { return t.identifier }

func (t *stopErrTask) Start(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (t *stopErrTask) Stop(ctx context.Context) error {
	time.Sleep(t.stopDelay)
	return t.stopErr
}

// TestTombstoneCapturesLateStopError covers the result recorded when Start
// returns before a concurrent Stop has settled: the retained result must be
// refreshed with the stop error rather than frozen at the Start-only snapshot.
func TestTombstoneCapturesLateStopError(t *testing.T) {
	m := NewManager()
	wantErr := errors.New("stop failed")
	task := &stopErrTask{identifier: "late", stopDelay: 50 * time.Millisecond, stopErr: wantErr}

	if err := m.StartTask(context.Background(), "late", task); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := m.StopTask(ctx, "late"); !errors.Is(err, wantErr) {
		t.Fatalf("StopTask error = %v, want %v", err, wantErr)
	}

	// The task is gone from the live map; the retained result must still carry
	// the stop error for a later query.
	result, ok := m.GetTaskResult("late")
	if !ok {
		t.Fatal("GetTaskResult should resolve a finished task from its tombstone")
	}
	if !errors.Is(result, wantErr) {
		t.Errorf("tombstone result = %v, want it to wrap %v", result, wantErr)
	}
}

// TestTombstonesAreBounded pins that starting tasks under ever-changing names
// cannot grow the retained-result map without bound.
func TestTombstonesAreBounded(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	for i := range maxTombstones + 100 {
		name := fmt.Sprintf("job-%d", i)
		if err := m.StartTask(ctx, name, &stopErrTask{identifier: name}); err != nil {
			t.Fatalf("StartTask %s: %v", name, err)
		}
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		if err := m.StopTask(stopCtx, name); err != nil {
			cancel()
			t.Fatalf("StopTask %s: %v", name, err)
		}
		cancel()
	}

	m.mu.RLock()
	size, order := len(m.tombstones), len(m.tombstoneOrder)
	m.mu.RUnlock()

	if size > maxTombstones {
		t.Errorf("tombstones grew to %d entries, want at most %d", size, maxTombstones)
	}
	if order != size {
		t.Errorf("tombstoneOrder (%d) drifted from tombstones (%d)", order, size)
	}
	// The most recent task must still be resolvable.
	if _, ok := m.GetTaskResult(fmt.Sprintf("job-%d", maxTombstones+99)); !ok {
		t.Error("the newest finished task should still be retained")
	}
}

// TestRestartUnderSameNameClearsTombstone pins that re-registering a name drops
// the previous run's result instead of letting a stale entry shadow the new run.
func TestRestartUnderSameNameClearsTombstone(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	wantErr := errors.New("first run failed")

	if err := m.StartTask(ctx, "recycled", &stopErrTask{identifier: "recycled", stopErr: wantErr}); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	_ = m.StopTask(stopCtx, "recycled")
	cancel()

	if err := m.StartTask(ctx, "recycled", &stopErrTask{identifier: "recycled"}); err != nil {
		t.Fatalf("restart: %v", err)
	}
	result, ok := m.GetTaskResult("recycled")
	if !ok {
		t.Fatal("restarted task should be resolvable")
	}
	if errors.Is(result, wantErr) {
		t.Error("the previous run's error leaked into the restarted task's result")
	}

	m.mu.RLock()
	order := len(m.tombstoneOrder)
	m.mu.RUnlock()
	if order != 0 {
		t.Errorf("tombstoneOrder retained %d stale entries after re-registration", order)
	}
}
