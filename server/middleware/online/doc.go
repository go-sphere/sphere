// Package online tracks which users or sessions were recently active. An
// [Online] tracker records a key per request through its httpx middleware,
// forgets keys after a TTL, and reports the approximate number of live keys.
//
// Build the tracker with [NewOnline], install [Online.Middleware] on the
// routes to observe, and run the tracker as a core/task.Task so its sweeper
// reclaims expired keys. Without Start the backing map grows without bound.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/httpx"
//		"github.com/go-sphere/sphere/core/boot"
//		"github.com/go-sphere/sphere/server/middleware/online"
//	)
//
//	tracker := online.NewOnline()
//	api.Use(tracker.Middleware(func(ctx httpx.Context) string {
//		return ctx.Header("X-Session-ID") // "" is not tracked
//	}, 5*time.Minute))
//
//	// Run it alongside the HTTP server; boot starts and stops both.
//	return boot.NewApplication(httpServer, tracker), nil
//
//	// Later, for example in a metrics handler:
//	count := tracker.OnlineCount()
package online
