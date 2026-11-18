package redistest

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisConn "github.com/go-sphere/sphere/infra/redis"
	"github.com/redis/go-redis/v9"
)

// NewTestRedisClient starts a miniredis server and returns a go-redis client
// pointed at it. A background ticker periodically fast-forwards miniredis by
// the elapsed wall-clock duration so Redis TTLs fire reliably even under load.
// tb.Cleanup stops the ticker and closes the client.
func NewTestRedisClient(tb testing.TB) *redis.Client {
	tb.Helper()

	mini := miniredis.RunT(tb)

	client, err := redisConn.NewClient(redisConn.Config{
		URL: "redis://" + mini.Addr() + "/0",
	})
	if err != nil {
		tb.Fatalf("failed to create redis client for miniredis: %v", err)
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		last := time.Now()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				if now.After(last) {
					mini.FastForward(now.Sub(last))
					last = now
				}
			}
		}
	}()

	tb.Cleanup(func() {
		close(done)
		// Join the ticker goroutine before tearing anything down: cleanups run
		// LIFO, so miniredis's own cleanup (server shutdown) follows this one
		// and must not race an in-flight FastForward.
		<-stopped
		_ = client.Close()
	})

	return client
}
