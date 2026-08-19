package task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// countingTask records how many times Stop was called and lets the test drive
// when Start returns.
type countingTask struct {
	identifier string
	release    chan struct{}
	startErr   error
	stopErr    error
	stopDelay  time.Duration
	stops      atomic.Int32
}

func (t *countingTask) Identifier() string { return t.identifier }

func (t *countingTask) Start(ctx context.Context) error {
	select {
	case <-t.release:
		return t.startErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *countingTask) Stop(context.Context) error {
	t.stops.Add(1)
	if t.stopDelay > 0 {
		time.Sleep(t.stopDelay)
	}
	return t.stopErr
}

// TestSelfExitedTaskIsStopped pins the contract alignment with Group: Stop is the
// cleanup half of the lifecycle and runs for every task, including one that
// returned from Start on its own without anyone asking it to shut down.
func TestSelfExitedTaskIsStopped(t *testing.T) {
	m := NewManager()
	task := &countingTask{identifier: "oneshot", release: make(chan struct{})}

	if err := m.StartTask(context.Background(), "oneshot", task); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	close(task.release) // Start returns on its own; nobody calls StopTask.

	if err := m.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := task.stops.Load(); got != 1 {
		t.Errorf("Stop called %d times for a self-exited task, want exactly 1", got)
	}
}

// TestSelfExitedFailingTaskIsStopped covers the leak-prone case: Start bailed out
// with an error, so whatever it had already allocated still needs releasing.
func TestSelfExitedFailingTaskIsStopped(t *testing.T) {
	m := NewManager()
	wantErr := errors.New("start blew up")
	task := &countingTask{identifier: "broken", release: make(chan struct{}), startErr: wantErr}

	if err := m.StartTask(context.Background(), "broken", task); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	close(task.release)

	if err := m.Wait(); !errors.Is(err, wantErr) {
		t.Fatalf("Wait error = %v, want it to wrap %v", err, wantErr)
	}
	if got := task.stops.Load(); got != 1 {
		t.Errorf("Stop called %d times, want exactly 1", got)
	}
	ok, result := m.GetTaskResult("broken")
	if !ok || !errors.Is(result, wantErr) {
		t.Errorf("GetTaskResult = (%v, %v), want the start error to be retained", ok, result)
	}
}

// TestExplicitStopDoesNotDoubleStop pins that the run goroutine's cleanup and an
// explicit StopTask collapse into a single Stop via stopOnce.
func TestExplicitStopDoesNotDoubleStop(t *testing.T) {
	m := NewManager()
	task := &countingTask{identifier: "svc", release: make(chan struct{})}

	if err := m.StartTask(context.Background(), "svc", task); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := m.StopTask(ctx, "svc"); err != nil {
		t.Fatalf("StopTask: %v", err)
	}
	if err := m.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := task.stops.Load(); got != 1 {
		t.Errorf("Stop called %d times, want exactly 1", got)
	}
}

// TestWaitBlocksUntilStopSettles pins the guarantee Wait now makes: it returns
// only after every task's Stop has finished, matching Group.Start.
func TestWaitBlocksUntilStopSettles(t *testing.T) {
	m := NewManager()
	task := &countingTask{
		identifier: "slowstop",
		release:    make(chan struct{}),
		stopDelay:  150 * time.Millisecond,
	}

	if err := m.StartTask(context.Background(), "slowstop", task); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	close(task.release)

	if err := m.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	// If Wait returned early the counter would still be 0 here.
	if got := task.stops.Load(); got != 1 {
		t.Errorf("Stop count %d after Wait returned; Wait did not await Stop", got)
	}
}

func TestDefaultCleanupTimeoutIsApplied(t *testing.T) {
	if got := NewManager().opts.cleanupTimeout; got != defaultManagerCleanupTimeout {
		t.Errorf("default cleanupTimeout = %v, want %v", got, defaultManagerCleanupTimeout)
	}
}

// TestExplicitZeroCleanupTimeoutOptsOut pins that the documented opt-out still
// works now that NewManager seeds a non-zero default.
func TestExplicitZeroCleanupTimeoutOptsOut(t *testing.T) {
	m := NewManager(WithManagerCleanupTimeout(0))
	if got := m.opts.cleanupTimeout; got != 0 {
		t.Fatalf("cleanupTimeout = %v, want 0 to disable the bound", got)
	}
	ctx, cancel := m.newCleanupContext()
	defer cancel()
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		t.Error("an opted-out cleanup context must not carry a deadline")
	}
}

// TestCleanupTimeoutBoundsStopContext pins that the configured budget actually
// reaches the ctx a task's Stop receives.
func TestCleanupTimeoutBoundsStopContext(t *testing.T) {
	m := NewManager(WithManagerCleanupTimeout(40 * time.Millisecond))
	observed := make(chan error, 1)

	task := scriptStopTask{
		identifier: "bounded",
		onStop: func(ctx context.Context) error {
			<-ctx.Done()
			observed <- ctx.Err()
			return nil
		},
	}

	if err := m.StartTask(context.Background(), "bounded", task); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if err := m.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	select {
	case err := <-observed:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("stop ctx error = %v, want DeadlineExceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop never observed its context expiring")
	}
}

// scriptStopTask exits Start immediately and runs a caller-supplied Stop.
type scriptStopTask struct {
	identifier string
	onStop     func(context.Context) error
}

func (t scriptStopTask) Identifier() string          { return t.identifier }
func (t scriptStopTask) Start(context.Context) error { return nil }

func (t scriptStopTask) Stop(ctx context.Context) error { return t.onStop(ctx) }
