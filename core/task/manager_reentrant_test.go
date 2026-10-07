package task

import (
	"context"
	"testing"
	"time"
)

// TestManagerTaskStartsSiblingDuringWait pins that a running task can call
// StartTask while Wait is blocked, and that Wait also waits for the sibling.
// Wait used to hold the registration lock, so this deadlocked.
func TestManagerTaskStartsSiblingDuringWait(t *testing.T) {
	m := NewManager()
	waiting := make(chan struct{})
	siblingDone := make(chan struct{})
	parent := NewFunc("parent", func(ctx context.Context) error {
		<-waiting
		return m.StartTask(ctx, "sibling", NewFunc("sibling", func(context.Context) error {
			time.Sleep(10 * time.Millisecond)
			close(siblingDone)
			return nil
		}, nil))
	}, nil)
	if err := m.StartTask(context.Background(), "parent", parent); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	waitErr := make(chan error, 1)
	go func() { waitErr <- m.Wait() }()
	time.Sleep(10 * time.Millisecond) // let Wait block before the sibling registers
	close(waiting)

	select {
	case err := <-waitErr:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait deadlocked on a task calling StartTask")
	}
	select {
	case <-siblingDone:
	default:
		t.Fatal("Wait returned before the sibling finished")
	}
}

// TestManagerStopStartsTaskDuringStopAll pins that a Stop calling StartTask
// does not deadlock StopAll.
func TestManagerStopStartsTaskDuringStopAll(t *testing.T) {
	m := NewManager()
	worker := NewFunc("worker", func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, func(context.Context) error {
		return m.StartTask(context.Background(), "followup", NewFunc("followup", nil, nil))
	})
	if err := m.StartTask(context.Background(), "worker", worker); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.StopAll(ctx); err != nil {
		t.Fatalf("StopAll: %v", err)
	}
	if err := m.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}
