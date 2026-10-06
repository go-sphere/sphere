package online

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

// headerContext builds a mock request context carrying the key header the
// tracker's keygen reads.
func headerContext(key, value string) *httpxmock.Context {
	return httpxmock.NewRequest(http.MethodGet, "/", nil, httpxmock.WithHeader(key, value))
}

// Online must stay a task.Task: its storage is only bounded while the periodic
// sweep is running, because the backing cache reclaims an expired entry when
// that key is read again or the map is swept, and the middleware only ever
// writes. Dropping the interface would silently reintroduce unbounded growth
// for a high-cardinality key such as a client IP.
var _ task.Task = (*Online)(nil)

func TestOnlineLifecycleContract(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		return NewOnline(WithTrimInterval(10 * time.Millisecond))
	})
}

// TestSweepReclaimsExpiredEntries checks that entries do not survive their TTL
// once the tracker is running. It observes through OnlineCount, while the
// periodic Trim (Start) is what physically reclaims; what it pins is the
// externally promised behaviour — an expired entry stops being counted — and,
// with the interface assertion above, that the sweep has somewhere to run.
func TestSweepReclaimsExpiredEntries(t *testing.T) {
	o := NewOnline(WithTrimInterval(10 * time.Millisecond))

	ctx := t.Context()
	go func() { _ = o.Start(ctx) }()
	t.Cleanup(func() { _ = o.Stop(context.Background()) })

	for _, key := range []string{"ip-1", "ip-2", "ip-3"} {
		if err := o.cache.SetWithTTL(ctx, key, struct{}{}, 20*time.Millisecond); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	if got := o.OnlineCount(); got != 3 {
		t.Fatalf("OnlineCount() = %d, want 3", got)
	}

	deadline := time.After(2 * time.Second)
	for {
		if o.OnlineCount() == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expired entries were still counted: %d", o.OnlineCount())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// TestZeroValueStartErrors covers an Online built via its zero value rather
// than NewOnline. Start must fail with ErrNotInitialized instead of panicking
// in time.NewTicker on the zero trim interval.
func TestZeroValueStartErrors(t *testing.T) {
	var o Online
	if err := o.Start(context.Background()); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Start on zero-value Online: got %v, want ErrNotInitialized", err)
	}
}

// TestZeroValueMiddlewareAndCountDoNotPanic covers the request-path methods on a
// zero-value Online. They previously dereferenced the nil backing cache and
// panicked mid-request; the zero value must fail diagnosably or report nothing
// instead.
func TestZeroValueMiddlewareAndCountDoNotPanic(t *testing.T) {
	var o Online

	if count := o.OnlineCount(); count != 0 {
		t.Fatalf("OnlineCount on zero-value Online = %d, want 0", count)
	}

	mw := o.Middleware(func(ctx httpx.Context) string { return "k" }, time.Minute)
	err := httpxmock.Run(httpxmock.New(nil), nil, mw)
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Middleware on zero-value Online: got %v, want ErrNotInitialized", err)
	}
}

// TestWithTrimIntervalIgnoresNonPositive pins that a bad interval keeps the
// default instead of panicking in time.NewTicker on Start.
func TestWithTrimIntervalIgnoresNonPositive(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		if got := NewOnline(WithTrimInterval(interval)).trimInterval; got != defaultTrimInterval {
			t.Errorf("WithTrimInterval(%v) = %v, want the default %v", interval, got, defaultTrimInterval)
		}
	}
}

func TestOnline_Identifier(t *testing.T) {
	t.Parallel()
	o := NewOnline()
	if got := o.Identifier(); got != "online" {
		t.Fatalf("o.Identifier() = %q, want online", got)
	}
}

func TestOnline_Middleware(t *testing.T) {
	t.Parallel()

	o := NewOnline()
	keygen := func(ctx httpx.Context) string {
		return ctx.Header("X-User-ID")
	}
	mw := o.Middleware(keygen, 10*time.Minute)

	// 1. Non-empty key records presence
	next1 := &httpxmock.Handler{}
	if err := httpxmock.Run(headerContext("X-User-ID", "user-1001"), next1.Handle, mw); err != nil {
		t.Fatalf("Run with a key: %v", err)
	}
	if !next1.Called() {
		t.Fatal("the chain did not continue on a valid key")
	}
	if count := o.OnlineCount(); count != 1 {
		t.Fatalf("OnlineCount() = %d, want 1", count)
	}

	// 2. Empty key does not record presence but proceeds
	next2 := &httpxmock.Handler{}
	if err := httpxmock.Run(httpxmock.New(nil), next2.Handle, mw); err != nil {
		t.Fatalf("Run without a key: %v", err)
	}
	if !next2.Called() {
		t.Fatal("the chain did not continue on an empty key")
	}
	if count := o.OnlineCount(); count != 1 {
		t.Fatalf("OnlineCount() = %d, want 1 (unchanged)", count)
	}
}

// TestOnline_AdversarialConcurrentPresenceAndTrimming tests 100+ concurrent requests
// registering presence while the background trimmer sweeps frequently, verifying zero
// race conditions, correct counts, and memory cleanup after TTL expiration.
func TestOnline_AdversarialConcurrentPresenceAndTrimming(t *testing.T) {
	t.Parallel()

	tracker := NewOnline(WithTrimInterval(10 * time.Millisecond))

	ctx := t.Context()

	// Start background trimmer
	trimmerDone := make(chan error, 1)
	go func() {
		trimmerDone <- tracker.Start(ctx)
	}()

	keygen := func(ctx httpx.Context) string {
		return ctx.Header("X-Session-ID")
	}

	// Short TTL so we can observe trimming
	const itemTTL = 50 * time.Millisecond
	mw := tracker.Middleware(keygen, itemTTL)

	const numWorkers = 50
	const requestsPerWorker = 20
	const totalKeys = numWorkers * requestsPerWorker

	var wg sync.WaitGroup
	var successCount atomic.Int64

	// Concurrent writes
	for w := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for r := range requestsPerWorker {
				key := fmt.Sprintf("session-%d-%d", workerID, r)
				next := &httpxmock.Handler{}
				err := httpxmock.Run(headerContext("X-Session-ID", key), next.Handle, mw)
				if err != nil {
					t.Errorf("worker %d req %d failed: %v", workerID, r, err)
					return
				}
				if !next.Called() {
					t.Errorf("worker %d req %d next not called", workerID, r)
					return
				}
				successCount.Add(1)
			}
		}(w)
	}

	wg.Wait()

	if success := successCount.Load(); success != int64(totalKeys) {
		t.Fatalf("successCount = %d, want %d", success, totalKeys)
	}

	// Immediately after writing, count should be positive (close to totalKeys)
	countAfterWrite := tracker.OnlineCount()
	if countAfterWrite == 0 {
		t.Fatalf("OnlineCount immediately after writing is 0, expected > 0")
	}

	// Wait for TTL to expire and background trimmer to prune all entries
	deadline := time.After(2 * time.Second)
	trimmed := false
	for !trimmed {
		select {
		case <-deadline:
			t.Fatalf("memory leak: tracker.OnlineCount() = %d after TTL expiration, want 0", tracker.OnlineCount())
		case <-time.After(20 * time.Millisecond):
			if tracker.OnlineCount() == 0 {
				trimmed = true
			}
		}
	}

	// Test Stop idempotency under concurrency
	var stopWg sync.WaitGroup
	for range 10 {
		stopWg.Go(func() {
			_ = tracker.Stop(context.Background())
		})
	}
	stopWg.Wait()

	// Wait for Start goroutine to finish cleanly
	select {
	case err := <-trimmerDone:
		if err != nil && err != context.Canceled {
			t.Errorf("Start returned unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("tracker.Start did not stop after tracker.Stop()")
	}
}

// TestZeroValueStopDoesNotPanic pins that Stop on a zero-value Online is safe.
// The task contract requires Stop to be safe when Start failed, and Start on
// the zero value fails with ErrNotInitialized; Stop previously closed the nil
// done channel and panicked.
func TestZeroValueStopDoesNotPanic(t *testing.T) {
	var o Online
	for range 2 {
		if err := o.Stop(context.Background()); err != nil {
			t.Fatalf("Stop on zero-value Online = %v, want nil", err)
		}
	}
}
