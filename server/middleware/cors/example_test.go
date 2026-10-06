package cors_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/middleware/cors"
)

func ExampleNewCORS() {
	corsMiddleware, err := cors.NewCORS(
		cors.WithAllowOrigins("https://*.example.com"),
		cors.WithAllowCredentials(true),
		cors.WithMaxAge(10*time.Minute),
	)
	if err != nil {
		fmt.Println("cors:", err)
		return
	}

	engine := stdx.New()
	engine.Use(corsMiddleware)
	engine.Group("").GET("/data", func(ctx httpx.Context) error {
		return ctx.Text(http.StatusOK, "data")
	})

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	send := func(method, origin string) {
		req := httptest.NewRequest(method, "/data", nil)
		req.Header.Set("Origin", origin)
		resp, err := tr.Do(req)
		if err != nil {
			fmt.Println("request:", err)
			return
		}
		_ = resp.Body.Close()
		fmt.Printf("%s %s -> %d allow-origin=%q max-age=%q\n", method, origin, resp.StatusCode,
			resp.Header.Get("Access-Control-Allow-Origin"), resp.Header.Get("Access-Control-Max-Age"))
	}

	send(http.MethodOptions, "https://app.example.com")
	send(http.MethodGet, "https://app.example.com")
	send(http.MethodGet, "https://evil.test")
	// Output:
	// OPTIONS https://app.example.com -> 204 allow-origin="https://app.example.com" max-age="600"
	// GET https://app.example.com -> 200 allow-origin="https://app.example.com" max-age="600"
	// GET https://evil.test -> 200 allow-origin="" max-age="600"
}

func ExampleNewCORS_wildcardWithCredentials() {
	_, err := cors.NewCORS(
		cors.WithAllowOrigins("*"),
		cors.WithAllowCredentials(true),
	)
	fmt.Println(errors.Is(err, cors.ErrWildcardWithCredentials))
	// Output:
	// true
}
