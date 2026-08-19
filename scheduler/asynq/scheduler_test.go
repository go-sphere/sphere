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

// TestCronKindCollidesWithHandle checks that a cron name and an async kind that
// map to the same mux pattern are rejected as duplicates instead of panicking
// inside asynq's ServeMux.
func TestCronKindCollidesWithHandle(t *testing.T) {
	t.Run("HandleThenRegister", func(t *testing.T) {
		s := newTestScheduler(t)
		if err := s.Handle(cronKind("report"), func(context.Context, []byte) error { return nil }); err != nil {
			t.Fatalf("handle: %v", err)
		}
		err := s.Register("report", "@every 1s", func(context.Context) error { return nil })
		if !errors.Is(err, scheduler.ErrDuplicateName) {
			t.Fatalf("register colliding name error = %v, want %v", err, scheduler.ErrDuplicateName)
		}
	})

	t.Run("RegisterThenHandle", func(t *testing.T) {
		s := newTestScheduler(t)
		if err := s.Register("report", "@every 1s", func(context.Context) error { return nil }); err != nil {
			t.Fatalf("register: %v", err)
		}
		err := s.Handle(cronKind("report"), func(context.Context, []byte) error { return nil })
		if !errors.Is(err, scheduler.ErrDuplicateName) {
			t.Fatalf("handle colliding kind error = %v, want %v", err, scheduler.ErrDuplicateName)
		}
	})
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

// TestUnregisteredKindIsNotPrefixRouted pins that routing is by exact kind.
//
// asynq's ServeMux falls back to longest-prefix matching when no pattern matches
// exactly. Resolving the handler from the matched pattern therefore delivered an
// unregistered kind to whichever registered kind happened to be a prefix of it,
// handing that handler a payload it was never meant to decode. Hierarchical
// names are the documented style, so versioning or subdividing a kind walked
// straight into it.
func TestUnregisteredKindIsNotPrefixRouted(t *testing.T) {
	s := newTestScheduler(t)
	delivered := make(chan []byte, 1)
	if err := s.Handle("user.email", func(_ context.Context, payload []byte) error {
		delivered <- payload
		return nil
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	startTestScheduler(t, s)

	if _, err := s.Enqueue(context.Background(), "user.email.welcome.v2", []byte("v2 payload")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case payload := <-delivered:
		t.Fatalf("unregistered kind was routed to the \"user.email\" handler with payload %q", payload)
	case <-time.After(1500 * time.Millisecond):
		// The task must not reach the prefix handler.
	}

	// The exact kind still routes normally.
	if _, err := s.Enqueue(context.Background(), "user.email", []byte("exact")); err != nil {
		t.Fatalf("enqueue exact: %v", err)
	}
	select {
	case payload := <-delivered:
		if string(payload) != "exact" {
			t.Fatalf("payload = %q, want exact", payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("the exactly matching kind was not handled")
	}
}

// TestHandleRejectsEmptyKind pins that a kind read from configuration cannot
// take the process down. asynq's ServeMux panics on a blank pattern, and Handle
// runs inside the application builder, so the panic surfaced as a startup crash
// instead of the error the nil-handler check already returns.
func TestHandleRejectsEmptyKind(t *testing.T) {
	for _, kind := range []string{"", "   ", "\t"} {
		s := newTestScheduler(t)
		if err := s.Handle(kind, func(context.Context, []byte) error { return nil }); err == nil {
			t.Fatalf("Handle(%q) = nil, want an error", kind)
		}
	}
}
