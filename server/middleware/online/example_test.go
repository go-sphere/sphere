package online_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/middleware/online"
)

func ExampleOnline() {
	tracker := online.NewOnline(online.WithTrimInterval(time.Minute))

	// Run the sweeper for the life of the server; core/boot usually does this.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tracker.Start(ctx) }()
	defer func() {
		if err := tracker.Stop(context.Background()); err != nil {
			fmt.Println("stop:", err)
		}
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			fmt.Println("start:", err)
		}
	}()

	engine := stdx.New()
	engine.Group("", tracker.Middleware(func(ctx httpx.Context) string {
		return ctx.Header("X-Session-ID")
	}, 5*time.Minute)).GET("/ping", func(ctx httpx.Context) error {
		return ctx.NoContent(http.StatusNoContent)
	})

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	for _, session := range []string{"s1", "s2", "s1", ""} {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("X-Session-ID", session)
		resp, err := tr.Do(req)
		if err != nil {
			fmt.Println("request:", err)
			return
		}
		_ = resp.Body.Close()
	}

	fmt.Println(tracker.OnlineCount())
	// Output:
	// 2
}
