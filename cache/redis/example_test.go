package redis_test

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-sphere/sphere/cache/redis"
	redisconn "github.com/go-sphere/sphere/infra/redis"
)

func ExampleNewByteCache() {
	ctx := context.Background()
	// miniredis stands in for a real Redis server in this example.
	srv, err := miniredis.Run()
	if err != nil {
		fmt.Println("miniredis:", err)
		return
	}
	defer srv.Close()

	client, err := redisconn.NewClient(redisconn.Config{URL: "redis://" + srv.Addr() + "/0"})
	if err != nil {
		fmt.Println("client:", err)
		return
	}
	defer func() { _ = client.Close() }() // NewByteCache does not close an injected client

	c := redis.NewByteCache(client)
	if err := c.SetWithTTL(ctx, "k", []byte("v"), time.Minute); err != nil {
		fmt.Println("set:", err)
		return
	}
	v, found, err := c.Get(ctx, "k")
	fmt.Println(string(v), found, err)
	// Output: v true <nil>
}
