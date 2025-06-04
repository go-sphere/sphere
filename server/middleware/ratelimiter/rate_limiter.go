package ratelimiter

import (
	"net/http"
	"time"

	"github.com/TBXark/sphere/cache/memory"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func NewRateLimiter(key func(*gin.Context) string, createLimiter func(*gin.Context) (*rate.Limiter, time.Duration), abort func(*gin.Context)) gin.HandlerFunc {
	limiterSet := memory.NewMemoryCache[*rate.Limiter]()
	return func(ctx *gin.Context) {
		k := key(ctx)
		limiter, _ := limiterSet.Get(ctx, k)
		if limiter == nil || *limiter == nil {
			var expire time.Duration
			newLimiter, expire := createLimiter(ctx)
			err := limiterSet.SetWithTTL(ctx, k, newLimiter, expire)
			if err != nil {
				abort(ctx)
				return
			}
			limiter = &newLimiter
		}
		ok := (*limiter).Allow()
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
