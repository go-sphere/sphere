package ratelimiter_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/server/middleware/ratelimiter"
	"golang.org/x/time/rate"
)

func ExampleNewRateLimiter() {
	// One bucket per API key: bursts of 2, refilled once per hour.
	limit := ratelimiter.NewRateLimiter(
		func(ctx httpx.Context) string {
			return "key:" + ctx.Header("X-API-Key")
		},
		func(httpx.Context) (*rate.Limiter, time.Duration) {
			return rate.NewLimiter(rate.Every(time.Hour), 2), 2 * time.Hour
		},
	)

	engine := stdx.New(stdx.WithErrorHandler(httpz.AbortWithJsonError))
	engine.Group("", limit).GET("/search", httpz.WithText(func(httpx.Context) (string, error) {
		return "results", nil
	}))

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	for _, key := range []string{"a", "a", "a", "b"} {
		req := httptest.NewRequest(http.MethodGet, "/search", nil)
		req.Header.Set("X-API-Key", key)
		resp, err := tr.Do(req)
		if err != nil {
			fmt.Println("request:", err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			fmt.Println("read:", err)
			return
		}
		fmt.Println(key, resp.StatusCode, string(body))
	}
	// Output:
	// a 200 results
	// a 200 results
	// a 429 {"success":false,"code":0,"message":"rate limit exceeded"}
	// b 200 results
}
