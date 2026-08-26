package ratelimiter

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
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
	for i := 0; i < 2; i++ {
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
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := &fakeRateLimitContext{}
			_ = mw(ctx)
		}()
	}
	wg.Wait()

	mu.Lock()
	created := limiterCreated
	mu.Unlock()

	if created != 1 {
		t.Fatalf("limiter created %d times concurrently, want 1 (singleflight)", created)
	}
}
