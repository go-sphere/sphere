// Package ratelimiter is per-key golang.org/x/time/rate.Limiter middleware
// stored in a cache.Cache, with singleflight on miss. Deny is HTTP 429.
//
// NewRateLimiterByClientIP keys on httpx.Context.ClientIP, which is only as
// trustworthy as the engine's proxy configuration. Prefer an authenticated
// user ID when possible. createLimiter's expire is the cache TTL of the
// limiter object, not the rate window.
package ratelimiter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/memory"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

type options struct {
	cache      cache.Cache[*rate.Limiter]
	setTimeout time.Duration
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		cache:      memory.NewMemoryCache[*rate.Limiter](),
		setTimeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// Option is a functional option for configuring the rate limiter middleware.
type Option func(*options)

// WithCache sets a custom cache implementation for storing rate limiters.
// The default cache is an in-memory cache.
//
// The limiter is stored as a live in-process object and never round-trips
// through serialization: rate.Limiter has no exported fields, so a codec-
// backed cache (cache.NewCodecCache / NewJsonCache, e.g. over redis) stores
// the JSON "{}" and every later request receives a zero limiter that answers
// 429. Use the default in-process cache; across instances, keep one limiter
// per instance or share state with a serializable design of your own.
func WithCache(cache cache.Cache[*rate.Limiter]) Option {
	return func(opts *options) {
		opts.cache = cache
	}
}

// WithSetTimeout sets the timeout for cache set operations.
// This prevents hanging when the cache backend is unresponsive.
func WithSetTimeout(timeout time.Duration) Option {
	return func(opts *options) {
		if timeout > 0 {
			opts.setTimeout = timeout
		}
	}
}

// cacheCtx returns the context for a cache operation performed inside the
// singleflight: detached from the triggering request, because one caller
// disconnecting must not fail limiter creation for the waiters that share the
// result, and bounded by the configured timeout so an unresponsive backend
// cannot hang it.
func (o *options) cacheCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), o.setTimeout)
}

// NewRateLimiter creates a new rate limiting middleware with customizable key extraction and limiter creation.
// It uses caching to store rate limiters per key and singleflight to prevent cache stampedes.
//
// createLimiter runs once per key, inside a singleflight whose result every
// concurrent waiter for that key shares, and it receives the triggering
// request's context. It must therefore not depend on that request's
// cancellation or deadline (values are fine): one caller disconnecting would
// otherwise fail limiter creation for every waiter. The subsequent cache write
// is detached from the request for the same reason.
func NewRateLimiter(key func(httpx.Context) string, createLimiter func(httpx.Context) (*rate.Limiter, time.Duration), options ...Option) httpx.Middleware {
	sf := singleflight.Group{}
	opts := newOptions(options...)
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			k := key(ctx)
			limiter, exist, gErr := opts.cache.Get(ctx.Context(), k)
			if gErr != nil {
				return httpx.InternalServerError(gErr)
			}
			// A JSON round-trip through a codec cache yields a zero limiter with
			// both Limit and Burst zero; a legitimately configured limiter can
			// have Burst()==0 alone (rate.Inf), so require both to be zero.
			if exist && limiter != nil && limiter.Limit() == 0 && limiter.Burst() == 0 {
				return httpx.InternalServerError(fmt.Errorf(
					"ratelimiter: cache returned a zero limiter for key %q: "+
						"rate.Limiter is not serializable, use an in-process cache",
					k))
			}
			if !exist || limiter == nil {
				value, nErr, _ := sf.Do(k, func() (any, error) {
					// Re-read under the flight. singleflight only deduplicates
					// overlapping calls, so a caller that read the cache just
					// before another flight's write can lead a second flight
					// once that one finished; without this check it would
					// create a second limiter for the key and reset its burst.
					// A new flight starts only after the previous one's write
					// returned, so the write is visible here.
					readCtx, cancelRead := opts.cacheCtx(ctx.Context())
					cached, ok, rErr := opts.cache.Get(readCtx, k)
					cancelRead()
					if rErr != nil {
						return nil, rErr
					}
					if ok && cached != nil {
						return cached, nil
					}
					newLimiter, expire := createLimiter(ctx)
					// Every concurrent waiter for this key shares this result, so
					// the cache write must not inherit the triggering request's
					// cancellation: its disconnect would fail all waiters.
					setCtx, cancel := opts.cacheCtx(ctx.Context())
					defer cancel()
					err := opts.cache.SetWithTTL(setCtx, k, newLimiter, expire)
					if err != nil {
						return nil, err
					}
					return newLimiter, nil
				})
				if nErr != nil {
					return httpx.InternalServerError(nErr)
				}
				typed, ok := value.(*rate.Limiter)
				if !ok || typed == nil {
					return httpx.InternalServerError(fmt.Errorf("ratelimiter: unexpected limiter type %T", value))
				}
				limiter = typed
			}
			ok := limiter.Allow()
			if !ok {
				return httpx.NewWithStatus(http.StatusTooManyRequests, "rate limit exceeded")
			}
			return next(ctx)
		}
	}
}

// NewRateLimiterByClientIP rate limits per client IP, keyed on
// httpx.Context.ClientIP.
//
// The key is only as trustworthy as the engine's proxy configuration.
// ClientIP is documented as best-effort and typically derives from
// X-Forwarded-For, which the client sets: an adapter that trusts every peer —
// gin, echo, and hertz do by default — lets every request claim a fresh IP and
// receive its own full burst, so the limit stops applying at all. Worse, each
// fabricated address takes a slot in the limiter cache, so the header becomes a
// way to grow it.
//
// Configure the engine's trusted proxies before relying on this
// (WithTrustedProxies is uniform across the adapters; stdx ignores forwarding
// headers unless it is set), or pass NewRateLimiter a key drawn from something
// the caller cannot forge, such as an authenticated user ID.
func NewRateLimiterByClientIP(limit time.Duration, burst int, expire time.Duration, options ...Option) httpx.Middleware {
	return NewRateLimiter(
		func(ctx httpx.Context) string {
			return ctx.ClientIP()
		},
		func(ctx httpx.Context) (*rate.Limiter, time.Duration) {
			return rate.NewLimiter(rate.Every(limit), burst), expire
		},
		options...,
	)
}
