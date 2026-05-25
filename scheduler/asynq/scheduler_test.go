package asynq

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/scheduler"
	"github.com/go-sphere/sphere/test/redistest"
	sasynq "github.com/hibiken/asynq"
)

func TestEnqueueHandleEndToEnd(t *testing.T) {
	s := newTestScheduler(t)
	got := make(chan string, 1)
	if err := s.Handle("email.welcome", func(_ context.Context, payload []byte) error {
		got <- string(payload)
		return nil
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	startTestScheduler(t, s)

	id, err := s.Enqueue(context.Background(), "email.welcome", []byte("hello"))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if id == "" {
		t.Fatalf("enqueue returned empty task id")
	}

	select {
	case payload := <-got:
		if payload != "hello" {
			t.Fatalf("payload = %q, want hello", payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("task was not handled")
	}
}

func TestEnqueueWithDelay(t *testing.T) {
	s := newTestScheduler(t)
	got := make(chan time.Time, 1)
	if err := s.Handle("delayed", func(context.Context, []byte) error {
		got <- time.Now()
		return nil
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	startTestScheduler(t, s)

	if _, err := s.Enqueue(context.Background(), "delayed", nil, scheduler.WithDelay(250*time.Millisecond)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-got:
	case <-time.After(3 * time.Second):
		t.Fatalf("delayed task was not handled")
	}
}

func TestEnqueueWithUniqueForMapsDuplicate(t *testing.T) {
	s := newTestScheduler(t)
	if _, err := s.Enqueue(context.Background(), "unique", nil, scheduler.WithTaskID("unique-id"), scheduler.WithUniqueFor(time.Minute)); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	if _, err := s.Enqueue(context.Background(), "unique", nil, scheduler.WithTaskID("unique-id"), scheduler.WithUniqueFor(time.Minute)); !errors.Is(err, scheduler.ErrDuplicateName) {
		t.Fatalf("enqueue duplicate error = %v, want %v", err, scheduler.ErrDuplicateName)
	}
}

func TestHandleAfterStartReturnsErrAfterStart(t *testing.T) {
	s := newTestScheduler(t)
	startTestScheduler(t, s)
	if err := s.Handle("late", func(context.Context, []byte) error { return nil }); !errors.Is(err, scheduler.ErrAfterStart) {
		t.Fatalf("handle after start error = %v, want %v", err, scheduler.ErrAfterStart)
	}
}

func TestCloseThenMethodsReturnClosed(t *testing.T) {
	s := newTestScheduler(t)
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := s.Enqueue(context.Background(), "closed", nil); !errors.Is(err, scheduler.ErrClosed) {
		t.Fatalf("enqueue after close error = %v, want %v", err, scheduler.ErrClosed)
	}
	if err := s.Handle("closed", func(context.Context, []byte) error { return nil }); !errors.Is(err, scheduler.ErrClosed) {
		t.Fatalf("handle after close error = %v, want %v", err, scheduler.ErrClosed)
	}
}

func TestWithClientDoesNotCloseRedisClient(t *testing.T) {
	client := redistest.NewTestRedisClient(t)
	s, err := NewScheduler(Config{}, WithClient(client))
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close scheduler: %v", err)
	}
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("shared redis client was closed: %v", err)
	}
}

func TestNewSchedulerRequiresRedisClient(t *testing.T) {
	if _, err := NewScheduler(Config{}); err == nil {
		t.Fatalf("NewScheduler without redis client error = nil, want error")
	}
}

func TestWithMaxRetryRetriesFailures(t *testing.T) {
	s := newTestScheduler(t)
	var calls atomic.Int32
	if err := s.Handle("retry", func(context.Context, []byte) error {
		calls.Add(1)
		return errors.New("boom")
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	startTestScheduler(t, s)

	if _, err := s.Enqueue(context.Background(), "retry", nil, scheduler.WithMaxRetry(2)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitFor(t, 10*time.Second, func() bool { return calls.Load() >= 3 })

	inspector := sasynq.NewInspectorFromRedisClient(s.options.client)
	t.Cleanup(func() { _ = inspector.Close() })
	waitFor(t, 3*time.Second, func() bool {
		tasks, err := inspector.ListArchivedTasks("default")
		if err != nil {
			return false
		}
		for _, task := range tasks {
			if task.Type == "retry" && task.Retried == 2 {
				return true
			}
		}
		return false
	})
}

func newTestScheduler(t *testing.T) *Scheduler {
	t.Helper()
	client := redistest.NewTestRedisClient(t)
	s, err := NewScheduler(Config{
		ShutdownTimeout: time.Second,
	}, WithClient(client), WithServerConfig(func(cfg *sasynq.Config) {
		cfg.TaskCheckInterval = 50 * time.Millisecond
		cfg.DelayedTaskCheckInterval = 50 * time.Millisecond
		cfg.RetryDelayFunc = func(int, error, *sasynq.Task) time.Duration {
			return 50 * time.Millisecond
		}
	}))
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func startTestScheduler(t *testing.T, s *Scheduler) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()
	waitFor(t, time.Second, func() bool { return s.state.Load() == stateRunning })
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = s.Stop(stopCtx)
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("start returned: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("scheduler did not stop")
		}
	})
}

func waitFor(tb testing.TB, timeout time.Duration, ok func() bool) {
	tb.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	tb.Fatalf("condition not met within %s", timeout)
}
