// Package redistest starts a miniredis and returns a go-redis client for
// tests. A background ticker fast-forwards 5ms every 5ms so Redis TTLs
// fire without waiting on wall-clock. It is not a real Redis. Cleanup
// closes the client.
package redistest

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisConn "github.com/go-sphere/sphere/infra/redis"
	"github.com/redis/go-redis/v9"
)

// NewTestRedisClient starts a miniredis server and returns a go-redis client
// pointed at it. A background ticker fast-forwards miniredis 5ms every 5ms so
// Redis TTLs fire without waiting on wall-clock. t.Cleanup closes the client.
func NewTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	mini := miniredis.RunT(t)

	client, err := redisConn.NewClient(redisConn.Config{
		URL: "redis://" + mini.Addr() + "/0",
	})
	if err != nil {
		t.Fatalf("failed to create redis client for miniredis: %v", err)
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mini.FastForward(5 * time.Millisecond)
			}
		}
	}()

	t.Cleanup(func() {
		close(done)
		_ = client.Close()
	})

	return client
}
