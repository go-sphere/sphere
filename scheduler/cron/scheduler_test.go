package cron

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestStopTimeoutKeepsHandlerCtxWhileAnotherStopWaits pins that one Stop
// caller's deadline does not cancel jobs another Stop is still waiting on;
// the handler context is cancelled only when the last waiter gives up.
func TestStopTimeoutKeepsHandlerCtxWhileAnotherStopWaits(t *testing.T) {
	s, err := NewScheduler(Config{Seconds: true})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	cancelled := make(chan struct{}, 1)
	if err := s.Register("job", "* * * * * *", func(ctx context.Context) error {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			cancelled <- struct{}{}
		case <-release:
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	startDone := make(chan error, 1)
	go func() { startDone <- s.Start(context.Background()) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not start")
	}

	longDone := make(chan error, 1)
	longCtx, longCancel := context.WithCancel(context.Background())
	defer longCancel()
	go func() { longDone <- s.Stop(longCtx) }()
	waitWaiters(t, s, 1)

	shortCtx, shortCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer shortCancel()
	if err := s.Stop(shortCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("short Stop = %v, want DeadlineExceeded", err)
	}
	select {
	case <-cancelled:
		t.Fatal("handler ctx cancelled while a longer Stop was still waiting")
	case <-time.After(100 * time.Millisecond):
	}

	longCancel()
	if err := <-longDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("long Stop = %v, want Canceled", err)
	}
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("handler ctx not cancelled after the last Stop gave up")
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("final Stop: %v", err)
	}
	<-startDone
}

func waitWaiters(t *testing.T, s *Scheduler, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		got := s.stopWaiters
		s.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("stop waiters did not reach %d", n)
}
