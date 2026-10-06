// Package ratelimiter provides httpx middleware that rate limits requests per
// key with a golang.org/x/time/rate token bucket. Denied requests fail with
// HTTP 429 "rate limit exceeded".
//
// [NewRateLimiter] takes a key function and a limiter factory; one limiter is
// created per key on first use and kept in an in-process cache for the TTL the
// factory returns. [NewRateLimiterByClientIP] is the shortcut keyed on the
// client IP.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/httpx"
//		"github.com/go-sphere/sphere/server/auth/authorizer"
//		"github.com/go-sphere/sphere/server/middleware/ratelimiter"
//		"golang.org/x/time/rate"
//	)
//
//	// 5 requests per second with bursts of 10, per authenticated user.
//	limit := ratelimiter.NewRateLimiter(
//		func(ctx httpx.Context) string {
//			uid, err := authorizer.ContextUtils[int64]{}.GetCurrentID(ctx.Context())
//			if err != nil {
//				return "ip:" + ctx.ClientIP()
//			}
//			return "uid:" + strconv.FormatInt(uid, 10)
//		},
//		func(httpx.Context) (*rate.Limiter, time.Duration) {
//			return rate.NewLimiter(5, 10), time.Hour // limiter, cache TTL
//		},
//	)
//	api.Use(limit)
//
// The TTL returned by the factory is how long a key's limiter stays cached
// after it is created, not the rate window; once it expires (or the cache
// evicts it) the next request for that key starts with a fresh, full bucket.
// Limits are per process: instances do not share state. Client IPs are only as trustworthy as the engine's trusted-proxy
// configuration; see [NewRateLimiterByClientIP].
package ratelimiter
