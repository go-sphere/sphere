package ratelimiter

import (
	"context"
	"errors"
	"fmt"
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

func (f *fakeRateLimitContext) Next() error {
	f.nexted = true
	return nil
}

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
		if err := mw(ctx); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		if !ctx.nexted {
			t.Fatalf("request %d did not proceed to Next()", i+1)
		}
	}

	// 3rd request immediately should be rejected
	ctx := &fakeRateLimitContext{}
	err := mw(ctx)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	code, status, _ := httpx.ParseError(err)
	if status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d (code=%d)", status, http.StatusTooManyRequests, code)
	}
	if ctx.nexted {
		t.Fatal("rate limited request must not proceed to Next()")
	}
}

func TestNewRateLimiterByClientIP(t *testing.T) {
	t.Parallel()

	mw := NewRateLimiterByClientIP(time.Second, 1, time.Minute)

	// User from IP 1.1.1.1
	ctx1 := &fakeRateLimitContext{clientIP: "1.1.1.1"}
	if err := mw(ctx1); err != nil {
		t.Fatalf("IP 1.1.1.1 first request failed: %v", err)
	}
	if !ctx1.nexted {
		t.Fatal("IP 1.1.1.1 first request did not proceed to Next()")
	}

	// IP 1.1.1.1 second request should be rate limited
	ctx1Second := &fakeRateLimitContext{clientIP: "1.1.1.1"}
	if err := mw(ctx1Second); err == nil {
		t.Fatal("IP 1.1.1.1 second request should be rate limited")
	}

	// User from IP 2.2.2.2 should have independent bucket and succeed
	ctx2 := &fakeRateLimitContext{clientIP: "2.2.2.2"}
	if err := mw(ctx2); err != nil {
		t.Fatalf("IP 2.2.2.2 request failed: %v", err)
	}
	if !ctx2.nexted {
		t.Fatal("IP 2.2.2.2 request did not proceed to Next()")
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
		err := mw(ctx)
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
		err := mw(ctx)
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
		err := mw(ctx)
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
			t.Fatal("guard-triggered request must not proceed to Next()")
		}
	})
}

func TestNewRateLimiterSingleflight(t *testing.T) {
	t.Parallel()

	var limiterCreated int
	var mu sync.Mutex

	mw := NewRateLimiter(
		func(ctx httpx.Context) string { return "shared-key" },
		func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
			mu.Lock()
			limiterCreated++
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			return rate.NewLimiter(rate.Inf, 100), time.Minute
		},
	)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			ctx := &fakeRateLimitContext{}
			_ = mw(ctx)
		})
	}
	wg.Wait()

	mu.Lock()
	created := limiterCreated
	mu.Unlock()

	if created != 1 {
		t.Fatalf("limiter created %d times concurrently, want 1 (singleflight)", created)
	}
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

func (s *stressRateLimitContext) Next() error {
	s.nexted.Store(true)
	return nil
}

// TestRateLimiter_ConcurrentSingleflightStampede tests 200 concurrent requests hitting
// the rate limiter on a cold cache for the same key.
// It verifies singleflight deduplication (createLimiter called once) and accurate burst enforcement.
func TestRateLimiter_ConcurrentSingleflightStampede(t *testing.T) {
	t.Parallel()

	const burst = 25
	const totalRequests = 200

	var createdCount atomic.Int64

	mw := NewRateLimiter(
		func(ctx httpx.Context) string {
			return "stampede-key"
		},
		func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
			createdCount.Add(1)
			// Small artificial delay to maximize concurrent stampede in singleflight
			time.Sleep(10 * time.Millisecond)
			return rate.NewLimiter(rate.Every(time.Minute), burst), time.Minute
		},
	)

	var allowedCount atomic.Int64
	var limitedCount atomic.Int64
	var errorCount atomic.Int64

	var wg sync.WaitGroup
	wg.Add(totalRequests)

	// Start barrier to release all goroutines simultaneously
	startBarrier := make(chan struct{})

	for range totalRequests {
		go func() {
			defer wg.Done()
			<-startBarrier

			ctx := &stressRateLimitContext{clientIP: "10.0.0.1"}
			err := mw(ctx)
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
					t.Errorf("nil error but Next() not called")
				}
			}
		}()
	}

	// Release all goroutines
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

// TestRateLimiter_HighCardinalityConcurrentStampede tests 50 distinct client IPs
// with 10 concurrent requests each (500 total concurrent requests released via barrier),
// ensuring singleflight deduplication per key and independent rate limit buckets.
func TestRateLimiter_HighCardinalityConcurrentStampede(t *testing.T) {
	t.Parallel()

	const numIPs = 50
	const requestsPerIP = 10
	const burst = 3

	var createdTotal atomic.Int64

	mw := NewRateLimiter(
		func(ctx httpx.Context) string {
			return ctx.ClientIP()
		},
		func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
			createdTotal.Add(1)
			time.Sleep(10 * time.Millisecond) // Ensures all concurrent requests per IP join singleflight
			return rate.NewLimiter(rate.Every(time.Minute), burst), time.Minute
		},
		WithCache(mcache.NewMapCache[*rate.Limiter]()),
	)

	var wg sync.WaitGroup
	var totalAllowed atomic.Int64
	var totalLimited atomic.Int64
	var totalErrors atomic.Int64

	startBarrier := make(chan struct{})

	for ipIdx := range numIPs {
		clientIP := fmt.Sprintf("192.168.2.%d", ipIdx)
		for reqIdx := range requestsPerIP {
			wg.Add(1)
			go func(ip string, reqNum int) {
				defer wg.Done()
				<-startBarrier

				ctx := &stressRateLimitContext{clientIP: ip}
				err := mw(ctx)
				if err != nil {
					_, status, _ := httpx.ParseError(err)
					if status == http.StatusTooManyRequests {
						totalLimited.Add(1)
					} else {
						totalErrors.Add(1)
					}
				} else {
					if ctx.nexted.Load() {
						totalAllowed.Add(1)
					}
				}
			}(clientIP, reqIdx)
		}
	}

	close(startBarrier)
	wg.Wait()

	if errs := totalErrors.Load(); errs != 0 {
		t.Fatalf("totalErrors = %d, expected 0", errs)
	}

	// Each IP must create exactly 1 limiter due to singleflight
	if created := createdTotal.Load(); created != int64(numIPs) {
		t.Fatalf("createdTotal = %d, expected %d (1 per IP)", created, numIPs)
	}

	// For each of the 50 IPs, exactly `burst` (3) requests should be allowed
	expectedAllowed := int64(numIPs * burst)
	expectedLimited := int64(numIPs * (requestsPerIP - burst))

	if totalAllowed.Load() != expectedAllowed {
		t.Fatalf("totalAllowed = %d, expected %d", totalAllowed.Load(), expectedAllowed)
	}
	if totalLimited.Load() != expectedLimited {
		t.Fatalf("totalLimited = %d, expected %d", totalLimited.Load(), expectedLimited)
	}
}

// TestRateLimiter_HighCardinalityConcurrency_DefaultCache tests 100 distinct client IPs
// hitting the rate limiter concurrently with the default memory cache.
func TestRateLimiter_HighCardinalityConcurrency_DefaultCache(t *testing.T) {
	t.Parallel()

	const numIPs = 100
	const requestsPerIP = 5
	const burst = 2

	mw := NewRateLimiterByClientIP(time.Minute, burst, time.Minute)

	var wg sync.WaitGroup
	var totalAllowed atomic.Int64
	var totalLimited atomic.Int64
	var totalErrors atomic.Int64

	for ipIdx := range numIPs {
		clientIP := fmt.Sprintf("10.200.1.%d", ipIdx)
		for reqIdx := range requestsPerIP {
			wg.Add(1)
			go func(ip string, reqNum int) {
				defer wg.Done()
				ctx := &stressRateLimitContext{clientIP: ip}
				err := mw(ctx)
				if err != nil {
					_, status, _ := httpx.ParseError(err)
					if status == http.StatusTooManyRequests {
						totalLimited.Add(1)
					} else {
						totalErrors.Add(1)
					}
				} else {
					if ctx.nexted.Load() {
						totalAllowed.Add(1)
					}
				}
			}(clientIP, reqIdx)
		}
	}

	wg.Wait()

	if errs := totalErrors.Load(); errs != 0 {
		t.Fatalf("totalErrors = %d, expected 0", errs)
	}

	total := totalAllowed.Load() + totalLimited.Load()
	if total != int64(numIPs*requestsPerIP) {
		t.Fatalf("total processed = %d, expected %d", total, numIPs*requestsPerIP)
	}

	// At least `numIPs * burst` requests should be allowed
	if totalAllowed.Load() < int64(numIPs*burst) {
		t.Fatalf("totalAllowed = %d, expected >= %d", totalAllowed.Load(), numIPs*burst)
	}
}

// TestRateLimiter_ReplenishmentAndExpiration tests token replenishment and cache TTL eviction.
func TestRateLimiter_ReplenishmentAndExpiration(t *testing.T) {
	t.Parallel()

	// 1 token every 50ms, burst 1, cache TTL 100ms
	mw := NewRateLimiter(
		func(ctx httpx.Context) string { return "replenish-key" },
		func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
			return rate.NewLimiter(rate.Every(50*time.Millisecond), 1), 100 * time.Millisecond
		},
		WithCache(mcache.NewMapCache[*rate.Limiter]()),
	)

	// 1. First request allowed
	ctx1 := &stressRateLimitContext{}
	if err := mw(ctx1); err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// 2. Immediate second request rejected
	ctx2 := &stressRateLimitContext{}
	if err := mw(ctx2); err == nil {
		t.Fatal("immediate second request should be rejected")
	}

	// 3. Sleep 60ms for replenishment -> allowed
	time.Sleep(60 * time.Millisecond)
	ctx3 := &stressRateLimitContext{}
	if err := mw(ctx3); err != nil {
		t.Fatalf("replenished request failed: %v", err)
	}

	// 4. Immediate fourth request rejected
	ctx4 := &stressRateLimitContext{}
	if err := mw(ctx4); err == nil {
		t.Fatal("immediate fourth request should be rejected")
	}

	// 5. Sleep 120ms to trigger cache TTL eviction -> new limiter created -> allowed
	time.Sleep(120 * time.Millisecond)
	ctx5 := &stressRateLimitContext{}
	if err := mw(ctx5); err != nil {
		t.Fatalf("post-TTL request failed: %v", err)
	}
}
