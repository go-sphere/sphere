package ratelimiter

import (
	"context"
	"net/http"
	"time"

	"github.com/TBXark/sphere/cache/mcache"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

func NewRateLimiter(key func(*gin.Context) string, createLimiter func(*gin.Context) (*rate.Limiter, time.Duration), abort func(*gin.Context)) gin.HandlerFunc {
	sf := singleflight.Group{}
	limiterSet := mcache.NewMapCache[*rate.Limiter]()
	return func(ctx *gin.Context) {
		k := key(ctx)
		limiter, gErr := limiterSet.Get(ctx, k)
		if gErr != nil {
			abort(ctx)
			return
		}
		if limiter == nil {
			value, nErr, _ := sf.Do(k, func() (interface{}, error) {
				newLimiter, expire := createLimiter(ctx)
				err := limiterSet.SetWithTTL(context.WithoutCancel(ctx), k, newLimiter, expire)
				if err != nil {
					return nil, err
				}
				limiterSet.Trim()
				return newLimiter, nil
			})
			if nErr != nil {
				abort(ctx)
				return
			}
			newLimiter := value.(*rate.Limiter)
			limiter = &newLimiter
		}
		rateLimiter := *limiter
		ok := rateLimiter.Allow()
		if !ok {
			abort(ctx)
			return
		}
		ctx.Next()
	}
}

func NewNewRateLimiterByClientIP(limit time.Duration, burst int, expire time.Duration) gin.HandlerFunc {
	return NewRateLimiter(func(ctx *gin.Context) string {
		return ctx.ClientIP()
	}, func(ctx *gin.Context) (*rate.Limiter, time.Duration) {
		return rate.NewLimiter(rate.Every(limit), burst), expire
	}, func(ctx *gin.Context) {
		ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"message": "too many requests",
		})
	})
}
