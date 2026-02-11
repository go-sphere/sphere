package redistest

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisConn "github.com/go-sphere/sphere/infra/redis"
	"github.com/redis/go-redis/v9"
)

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
