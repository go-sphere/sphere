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
	cache  cache.Cache[*rate.Limiter]
	setTTL time.Duration
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		cache:  memory.NewMemoryCache[*rate.Limiter](),
		setTTL: 5 * time.Minute,
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
func WithCache(cache cache.Cache[*rate.Limiter]) Option {
	return func(opts *options) {
		opts.cache = cache
	}
}

// WithSetTTL sets the timeout for cache set operations.
// This prevents hanging when the cache backend is unresponsive.
func WithSetTTL(ttl time.Duration) Option {
	return func(opts *options) {
		opts.setTTL = ttl
	}
}

// NewRateLimiter creates a new rate limiting middleware with customizable key extraction and limiter creation.
// It uses caching to store rate limiters per key and singleflight to prevent cache stampedes.
func NewRateLimiter(key func(httpx.Context) string, createLimiter func(httpx.Context) (*rate.Limiter, time.Duration), options ...Option) httpx.Middleware {
	sf := singleflight.Group{}
	opts := newOptions(options...)
	return func(ctx httpx.Context) error {
		k := key(ctx)
		limiter, exist, gErr := opts.cache.Get(ctx.Context(), k)
		if gErr != nil {
			return httpx.InternalServerError(gErr)
		}
		if !exist || limiter == nil {
			value, nErr, _ := sf.Do(k, func() (any, error) {
				newLimiter, expire := createLimiter(ctx)
				setCtx, cancel := context.WithTimeout(ctx.Context(), opts.setTTL)
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
		return ctx.Next()
	}
}

// NewNewRateLimiterByClientIP rate limits per client IP, keyed on
// httpx.Context.ClientIP.
//
// The key is only as trustworthy as the engine's proxy configuration.
// ClientIP is documented as best-effort and typically derives from
// X-Forwarded-For, which the client sets: with the engine trusting all proxies —
// gin's default — every request can claim a fresh IP and receive its own full
// burst, so the limit stops applying at all. Worse, each fabricated address
// takes a slot in the limiter cache, so the header becomes a way to grow it.
//
// Configure the engine's trusted proxies before relying on this (gin
// SetTrustedProxies, echo IPExtractor, fiber EnableTrustedProxyCheck), or pass
// NewRateLimiter a key drawn from something the caller cannot forge, such as an
// authenticated user ID.
func NewNewRateLimiterByClientIP(limit time.Duration, burst int, expire time.Duration, options ...Option) httpx.Middleware {
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
