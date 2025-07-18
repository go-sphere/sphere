package ratelimiter

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/cache/memory"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

type options struct {
	abortWithError func(ctx *gin.Context, status int, err error)
	cache          cache.Cache[*rate.Limiter]
	setTTL         time.Duration
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		abortWithError: func(ctx *gin.Context, status int, err error) {
			ctx.AbortWithStatusJSON(status, gin.H{
				"error": err.Error(),
			})
		},
		cache:  memory.NewMemoryCache[*rate.Limiter](),
		setTTL: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

type Option func(*options)

func WithAbort(fn func(ctx *gin.Context, status int, err error)) Option {
	return func(opts *options) {
		opts.abortWithError = fn
	}
}

func WithCache(cache cache.Cache[*rate.Limiter]) Option {
	return func(opts *options) {
		opts.cache = cache
	}
}

func WithSetTTL(ttl time.Duration) Option {
	return func(opts *options) {
		opts.setTTL = ttl
	}
}

func NewRateLimiter(key func(*gin.Context) string, createLimiter func(*gin.Context) (*rate.Limiter, time.Duration), options ...Option) gin.HandlerFunc {
	sf := singleflight.Group{}
	opts := newOptions(options...)
	return func(ctx *gin.Context) {
		k := key(ctx)
		limiter, exist, gErr := opts.cache.Get(ctx, k)
		if gErr != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, gErr)
			return
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
				return
			}
			limiter = value.(*rate.Limiter)
		}
		ok := limiter.Allow()
		if !ok {
			opts.abortWithError(ctx, http.StatusTooManyRequests, errors.New("rate limit exceeded"))
			return
		}
		ctx.Next()
	}
}

func NewNewRateLimiterByClientIP(limit time.Duration, burst int, expire time.Duration, options ...Option) gin.HandlerFunc {
	return NewRateLimiter(
		func(ctx *gin.Context) string {
			return ctx.ClientIP()
		},
		func(ctx *gin.Context) (*rate.Limiter, time.Duration) {
			return rate.NewLimiter(rate.Every(limit), burst), expire
		},
		options...,
	)
}
