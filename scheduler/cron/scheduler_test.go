package cron

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/sphere/log"
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
	releaseOnce := sync.OnceFunc(func() { close(release) })
	defer releaseOnce()
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
	if err := <-startDone; err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
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

// TestWithLoggerNilIsIgnored pins that WithLogger(nil) keeps the default
// adapter. robfig's SkipIfStillRunning logs outside any recover in the job
// goroutine, so a nil logger would crash the process on the first overlap.
func TestWithLoggerNilIsIgnored(t *testing.T) {
	o := options{logger: cronLogger{}}
	WithLogger(nil)(&o)
	if o.logger == nil {
		t.Fatal("WithLogger(nil) replaced the default logger with nil")
	}
}

type countLogBackend struct {
	mu       sync.Mutex
	messages []string
}

func (b *countLogBackend) Log(_ context.Context, _ log.Level, msg string, _ ...log.Attr) {
	b.mu.Lock()
	b.messages = append(b.messages, msg)
	b.mu.Unlock()
}

func (b *countLogBackend) Sync() error                    { return nil }
func (b *countLogBackend) With(...log.Option) log.Backend { return b }

func (b *countLogBackend) count(msg string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, m := range b.messages {
		if m == msg {
			n++
		}
	}
	return n
}

func TestCronPanicLogsOnceAndStopWithExpiredCtxAfterDrain(t *testing.T) {
	recorder := &countLogBackend{}
	prev := log.With().Backend()
	log.InitWithBackends(recorder)
	t.Cleanup(func() { log.InitWithBackends(prev) })

	s, err := NewScheduler(Config{Seconds: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ran := make(chan struct{})
	once := sync.OnceFunc(func() { close(ran) })
	if err := s.Register("panic-once", "* * * * * *", func(context.Context) error {
		defer once()
		panic("boom")
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startDone := make(chan error, 1)
	go func() { startDone <- s.Start(ctx) }()

	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatal("cron job did not run")
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := <-startDone; err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	// Allow log delivery from deferred recover to settle, then verify no duplicate
	// "cron handler failed" line accompanied the recovered panic.
	if got := recorder.count("panic recovered"); got != 1 {
		t.Fatalf("panic recovered logs = %d, want 1", got)
	}
	if got := recorder.count("cron handler failed"); got != 0 {
		t.Fatalf("cron handler failed logs = %d, want 0 for recovered panic", got)
	}

	// Simulate an already-drained shutdown where Stop sees a done stopDone channel
	// together with an expired caller context.
	drainedStopDone := make(chan struct{})
	close(drainedStopDone)
	s2, err := NewScheduler(Config{})
	if err != nil {
		t.Fatal(err)
	}
	s2.state.Store(stateStopping)
	s2.stopDone = drainedStopDone
	expiredCtx, expiredCancel := context.WithCancel(context.Background())
	expiredCancel()
	if err := s2.Stop(expiredCtx); err != nil {
		t.Fatalf("Stop with expired ctx when stopDone already closed = %v, want nil", err)
	}
}
