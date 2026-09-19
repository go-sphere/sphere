package ratelimiter

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
	"golang.org/x/time/rate"
)

type httpxContext = httpx.Context

type fakeRateLimitContext struct {
	httpxContext
	ctx      context.Context
	clientIP string
	nexted   bool
}

func (f *fakeRateLimitContext) Context() context.Context {
	if f.ctx == nil {
		return context.Background()
	}
	return f.ctx
}

func (f *fakeRateLimitContext) ClientIP() string {
	return f.clientIP
}

func (f *fakeRateLimitContext) markNext() { f.nexted = true }

func TestNewRateLimiter(t *testing.T) {
	t.Parallel()

	// 1 token per 100ms, burst 2
	mw := NewRateLimiter(
		func(ctx httpx.Context) string {
			return "fixed-key"
		},
		func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
			return rate.NewLimiter(rate.Every(100*time.Millisecond), 2), time.Minute
		},
		WithSetTimeout(time.Second),
	)

	// First 2 requests should be allowed (burst 2)
	for i := range 2 {
		ctx := &fakeRateLimitContext{}
		if err := run(mw, ctx); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		if !ctx.nexted {
			t.Fatalf("request %d did not proceed down the chain", i+1)
		}
	}

	// 3rd request immediately should be rejected
	ctx := &fakeRateLimitContext{}
	err := run(mw, ctx)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	code, status, _ := httpx.ParseError(err)
	if status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d (code=%d)", status, http.StatusTooManyRequests, code)
	}
	if ctx.nexted {
		t.Fatal("rate limited request must not proceed down the chain")
	}
}

func TestNewRateLimiterByClientIP(t *testing.T) {
	t.Parallel()

	mw := NewRateLimiterByClientIP(time.Second, 1, time.Minute)

	// User from IP 1.1.1.1
	ctx1 := &fakeRateLimitContext{clientIP: "1.1.1.1"}
	if err := run(mw, ctx1); err != nil {
		t.Fatalf("IP 1.1.1.1 first request failed: %v", err)
	}
	if !ctx1.nexted {
		t.Fatal("IP 1.1.1.1 first request did not proceed down the chain")
	}

	// IP 1.1.1.1 second request should be rate limited
	ctx1Second := &fakeRateLimitContext{clientIP: "1.1.1.1"}
	if err := run(mw, ctx1Second); err == nil {
		t.Fatal("IP 1.1.1.1 second request should be rate limited")
	}

	// User from IP 2.2.2.2 should have independent bucket and succeed
	ctx2 := &fakeRateLimitContext{clientIP: "2.2.2.2"}
	if err := run(mw, ctx2); err != nil {
		t.Fatalf("IP 2.2.2.2 request failed: %v", err)
	}
	if !ctx2.nexted {
		t.Fatal("IP 2.2.2.2 request did not proceed down the chain")
	}
}

type errCache struct {
	cache.Cache[*rate.Limiter]
	getErr error
	setErr error
	getVal *rate.Limiter
}

func (e *errCache) Get(ctx context.Context, key string) (*rate.Limiter, bool, error) {
	if e.getErr != nil {
		return nil, false, e.getErr
	}
	if e.getVal != nil {
		return e.getVal, true, nil
	}
	return nil, false, nil
}

func (e *errCache) SetWithTTL(ctx context.Context, key string, val *rate.Limiter, ttl time.Duration) error {
	if e.setErr != nil {
		return e.setErr
	}
	return nil
}

func TestNewRateLimiterCacheErrors(t *testing.T) {
	t.Parallel()

	t.Run("get error returns 500", func(t *testing.T) {
		customErr := errors.New("redis connection down")
		mw := NewRateLimiter(
			func(ctx httpx.Context) string { return "k" },
			func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
				return rate.NewLimiter(rate.Inf, 1), time.Minute
			},
			WithCache(&errCache{getErr: customErr}),
		)

		ctx := &fakeRateLimitContext{}
		err := run(mw, ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		_, status, _ := httpx.ParseError(err)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", status)
		}
	})

	t.Run("set error returns 500", func(t *testing.T) {
		customErr := errors.New("cache write failed")
		mw := NewRateLimiter(
			func(ctx httpx.Context) string { return "k" },
			func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
				return rate.NewLimiter(rate.Inf, 1), time.Minute
			},
			WithCache(&errCache{setErr: customErr}),
		)

		ctx := &fakeRateLimitContext{}
		err := run(mw, ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		_, status, _ := httpx.ParseError(err)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", status)
		}
	})

	t.Run("zero limiter from a serializing cache returns 500", func(t *testing.T) {
		// A codec-backed cache stores the JSON "{}" for rate.Limiter (no
		// exported fields), which decodes to a zero limiter whose Allow is
		// always false. The middleware must surface that as an error naming
		// the misconfiguration rather than answering 429 for every request.
		mw := NewRateLimiter(
			func(ctx httpx.Context) string { return "k" },
			func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
				return rate.NewLimiter(rate.Inf, 1), time.Minute
			},
			WithCache(&errCache{getVal: &rate.Limiter{}}),
		)

		ctx := &fakeRateLimitContext{}
		err := run(mw, ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not serializable") {
			t.Fatalf("error = %v, want a message naming the serialization problem", err)
		}
		_, status, _ := httpx.ParseError(err)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", status)
		}
		if ctx.nexted {
			t.Fatal("guard-triggered request must not proceed down the chain")
		}
	})
}

type stressRateLimitContext struct {
	httpxContext
	ctx      context.Context
	clientIP string
	nexted   atomic.Bool
}

func (s *stressRateLimitContext) Context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *stressRateLimitContext) ClientIP() string {
	return s.clientIP
}

func (s *stressRateLimitContext) markNext() { s.nexted.Store(true) }

// nextRecorder is what both fake contexts share: the flag the terminal handler
// sets when the chain reaches it.
type nextRecorder interface {
	httpx.Context
	markNext()
}

// run drives mw with a terminal handler that records whether the chain
// continued past the middleware.
func run[C nextRecorder](mw httpx.Middleware, ctx C) error {
	return mw(func(httpx.Context) error {
		ctx.markNext()
		return nil
	})(ctx)
}

// TestRateLimiter_ConcurrentSingleflightStampede verifies that concurrent cache
// misses for one key create a single limiter and share its burst budget.
func TestRateLimiter_ConcurrentSingleflightStampede(t *testing.T) {
	t.Parallel()

	const burst = 8
	const totalRequests = 64

	var createdCount atomic.Int64
	var arrived atomic.Int64
	allArrived := make(chan struct{})

	mw := NewRateLimiter(
		func(ctx httpx.Context) string {
			if arrived.Add(1) == totalRequests {
				close(allArrived)
			}
			return "stampede-key"
		},
		func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
			createdCount.Add(1)
			<-allArrived
			return rate.NewLimiter(rate.Every(time.Minute), burst), time.Minute
		},
	)

	var allowedCount atomic.Int64
	var limitedCount atomic.Int64
	var errorCount atomic.Int64

	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	for range totalRequests {
		wg.Go(func() {
			<-startBarrier

			ctx := &stressRateLimitContext{clientIP: "10.0.0.1"}
			err := run(mw, ctx)
			if err != nil {
				_, status, _ := httpx.ParseError(err)
				if status == http.StatusTooManyRequests {
					limitedCount.Add(1)
				} else {
					errorCount.Add(1)
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if ctx.nexted.Load() {
					allowedCount.Add(1)
				} else {
					t.Errorf("nil error but the chain did not continue")
				}
			}
		})
	}

	close(startBarrier)
	wg.Wait()

	if created := createdCount.Load(); created != 1 {
		t.Fatalf("createLimiter called %d times, expected 1 (singleflight failure)", created)
	}

	if allowed := allowedCount.Load(); allowed != int64(burst) {
		t.Fatalf("allowedCount = %d, expected exactly burst %d", allowed, burst)
	}

	if limited := limitedCount.Load(); limited != int64(totalRequests-burst) {
		t.Fatalf("limitedCount = %d, expected %d", limited, totalRequests-burst)
	}

	if errs := errorCount.Load(); errs != 0 {
		t.Fatalf("errorCount = %d, expected 0", errs)
	}
}

// recheckCache scripts the reads one request makes on the miss path: the first
// answers a miss (the request read the cache just before another flight's
// write), every later read answers the value that write stored, which is what a
// real cache returns once the write has landed. Writes fail the test: the
// flight must have used the cached limiter instead of creating a second one.
type recheckCache struct {
	cache.Cache[*rate.Limiter]
	reads atomic.Int64
	hit   *rate.Limiter
}

func (c *recheckCache) Get(context.Context, string) (*rate.Limiter, bool, error) {
	if c.reads.Add(1) == 1 {
		return nil, false, nil
	}
	return c.hit, true, nil
}

func (c *recheckCache) SetWithTTL(context.Context, string, *rate.Limiter, time.Duration) error {
	return errors.New("cache write: the flight created a limiter for a key another flight had just cached")
}

// TestRateLimiter_FlightRechecksCacheBeforeCreating pins that a singleflight
// re-reads the cache before calling createLimiter. A caller that read the cache
// just before another flight's write misses it, and singleflight does not
// deduplicate against a flight that already finished, so without the re-read it
// creates a second limiter for the key and resets its burst.
func TestRateLimiter_FlightRechecksCacheBeforeCreating(t *testing.T) {
	t.Parallel()

	var createdCount atomic.Int64
	c := &recheckCache{hit: rate.NewLimiter(rate.Every(time.Minute), 1)}
	mw := NewRateLimiter(
		func(httpx.Context) string { return "recheck-key" },
		func(httpx.Context) (*rate.Limiter, time.Duration) {
			createdCount.Add(1)
			return rate.NewLimiter(rate.Every(time.Minute), 100), time.Minute
		},
		WithCache(c),
	)

	first := &fakeRateLimitContext{}
	if err := run(mw, first); err != nil {
		t.Fatalf("middleware returned %v, want the cached limiter", err)
	}
	if created := createdCount.Load(); created != 0 {
		t.Fatalf("createLimiter called %d times, want 0: the flight must re-read the cache", created)
	}

	// The second request proves the burst was not reset: a freshly created
	// limiter (burst 100) would allow it, the cached one (burst 1) does not.
	second := &fakeRateLimitContext{}
	err := run(mw, second)
	if err == nil {
		t.Fatal("second request must be rate limited by the cached limiter")
	}
	_, status, _ := httpx.ParseError(err)
	if status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_CacheExpirationRebuildsLimiter(t *testing.T) {
	t.Parallel()

	var createdCount atomic.Int64
	mw := NewRateLimiter(
		func(ctx httpx.Context) string { return "expiration-key" },
		func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
			createdCount.Add(1)
			return rate.NewLimiter(rate.Every(time.Hour), 1), 100 * time.Millisecond
		},
		WithCache(mcache.NewMapCache[*rate.Limiter]()),
	)

	ctx1 := &stressRateLimitContext{}
	if err := run(mw, ctx1); err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	ctx2 := &stressRateLimitContext{}
	if err := run(mw, ctx2); err == nil {
		t.Fatal("immediate second request should be rejected")
	}

	time.Sleep(150 * time.Millisecond)
	ctx3 := &stressRateLimitContext{}
	if err := run(mw, ctx3); err != nil {
		t.Fatalf("post-TTL request failed: %v", err)
	}
	if created := createdCount.Load(); created != 2 {
		t.Fatalf("createLimiter called %d times, want 2", created)
	}
}

// cancelAwareRateCache is a cache whose writes honor cancellation and record
// what the write context looked like.
type cancelAwareRateCache struct {
	cache.Cache[*rate.Limiter]
	seenCtxErr error
}

func (c *cancelAwareRateCache) Get(context.Context, string) (*rate.Limiter, bool, error) {
	return nil, false, nil
}

func (c *cancelAwareRateCache) SetWithTTL(ctx context.Context, _ string, _ *rate.Limiter, _ time.Duration) error {
	// Let the triggering request be canceled while the write is in flight,
	// which is the situation that failed every singleflight waiter.
	time.Sleep(20 * time.Millisecond)
	c.seenCtxErr = ctx.Err()
	return c.seenCtxErr
}

// TestRateLimiter_CacheWriteOutlivesCanceledRequest pins that the cache write
// shared by all singleflight waiters does not inherit the triggering request's
// cancellation: one client disconnecting must not turn into a 500 for every
// other waiter on the key.
func TestRateLimiter_CacheWriteOutlivesCanceledRequest(t *testing.T) {
	// Not parallel: this test sleeps inside a cache write.
	c := &cancelAwareRateCache{}
	mw := NewRateLimiter(
		func(httpx.Context) string { return "cancel-key" },
		func(httpx.Context) (*rate.Limiter, time.Duration) {
			return rate.NewLimiter(rate.Every(time.Minute), 1), time.Minute
		},
		WithCache(c),
		WithSetTimeout(time.Second),
	)

	reqCtx, cancel := context.WithCancel(context.Background())
	ctx := &fakeRateLimitContext{ctx: reqCtx}
	cancel() // the triggering client is already gone

	if err := run(mw, ctx); err != nil {
		t.Fatalf("middleware returned %v, want the shared limiter", err)
	}
	if !ctx.nexted {
		t.Fatal("request must proceed down the chain")
	}
	if c.seenCtxErr != nil {
		t.Fatalf("cache write saw %v, want a context detached from the request", c.seenCtxErr)
	}
}

// TestRateLimiter_CacheHitAcceptsInfiniteLimiterWithZeroBurst pins that the
// zero-limiter guard does not reject a legitimately unlimited limiter:
// rate.NewLimiter(rate.Inf, 0) has Burst()==0 but Allow() is always true.
func TestRateLimiter_CacheHitAcceptsInfiniteLimiterWithZeroBurst(t *testing.T) {
	mw := NewRateLimiter(
		func(httpx.Context) string { return "inf-key" },
		func(httpx.Context) (*rate.Limiter, time.Duration) {
			t.Fatal("cache hit must not create a limiter")
			return nil, 0
		},
		WithCache(&errCache{getVal: rate.NewLimiter(rate.Inf, 0)}),
	)

	ctx := &fakeRateLimitContext{}
	if err := run(mw, ctx); err != nil {
		t.Fatalf("middleware returned %v, want the cached infinite limiter", err)
	}
	if !ctx.nexted {
		t.Fatal("request must proceed down the chain")
	}
}
