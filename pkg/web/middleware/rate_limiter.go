package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"golang.org/x/time/rate"
	"net/http"
	"time"
)

func NewRateLimiter(key func(*gin.Context) string, createLimiter func(*gin.Context) (*rate.Limiter, time.Duration), abort func(*gin.Context)) gin.HandlerFunc {
	limiterSet := cache.New(5*time.Minute, 10*time.Minute)
	return func(c *gin.Context) {
		k := key(c)
		limiter, ok := limiterSet.Get(k)
		if !ok {
			var expire time.Duration
			limiter, expire = createLimiter(c)
			limiterSet.Set(k, limiter, expire)
		}
		ok = limiter.(*rate.Limiter).Allow()
		if !ok {
			abort(c)
			return
		}
		c.Next()
	}
}

func NewNewRateLimiterByClientIP(limit time.Duration, burst int, expire time.Duration) gin.HandlerFunc {
	return NewRateLimiter(func(g *gin.Context) string {
		return g.ClientIP()
	}, func(g *gin.Context) (*rate.Limiter, time.Duration) {
		return rate.NewLimiter(rate.Every(limit), burst), expire
	}, func(g *gin.Context) {
		g.JSON(http.StatusTooManyRequests, gin.H{
			"message": "too many requests",
		})
	})
}
