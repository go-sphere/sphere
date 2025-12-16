package ratelimiter

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/memory"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

type options struct {
	abortWithError func(ctx httpx.Context, status int, err error)
	cache          cache.Cache[*rate.Limiter]
	setTTL         time.Duration
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		abortWithError: func(ctx httpx.Context, status int, err error) {
			ctx.JSON(status, httpx.H{
				"error": err.Error(),
			})
			ctx.Abort()
		},
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

// WithAbort sets a custom error handler for rate limit violations.
func WithAbort(fn func(ctx httpx.Context, status int, err error)) Option {
	return func(opts *options) {
		opts.abortWithError = fn
	}
}

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
		limiter, exist, gErr := opts.cache.Get(ctx, k)
		if gErr != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, gErr)
			return nil
		}
		if !exist || limiter == nil {
			value, nErr, _ := sf.Do(k, func() (interface{}, error) {
				newLimiter, expire := createLimiter(ctx)
				setCtx, cancel := context.WithTimeout(context.Background(), opts.setTTL)
				defer cancel()
				err := opts.cache.SetWithTTL(setCtx, k, newLimiter, expire)
				if err != nil {
					return nil, err
				}
				return newLimiter, nil
			})
			if nErr != nil {
				opts.abortWithError(ctx, http.StatusInternalServerError, gErr)
				return nil
			}
			limiter = value.(*rate.Limiter)
		}
		ok := limiter.Allow()
		if !ok {
			opts.abortWithError(ctx, http.StatusTooManyRequests, errors.New("rate limit exceeded"))
			return nil
		}
		return ctx.Next()
	}
}

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
