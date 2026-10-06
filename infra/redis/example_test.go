package redis_test

import (
	"context"
	"fmt"

	"github.com/alicebob/miniredis/v2"
	redisconn "github.com/go-sphere/sphere/infra/redis"
)

func ExampleNewClient() {
	// miniredis stands in for a real Redis server in this example.
	srv, err := miniredis.Run()
	if err != nil {
		fmt.Println("miniredis:", err)
		return
	}
	defer srv.Close()

	client, err := redisconn.NewClient(redisconn.Config{URL: "redis://" + srv.Addr() + "/0"})
	if err != nil {
		fmt.Println("invalid url:", err)
		return
	}
	defer func() { _ = client.Close() }()

	// NewClient does not connect; the first command does.
	fmt.Println(client.Ping(context.Background()).Val())
	// Output: PONG
}

// An unparsable URL fails at construction; the error does not echo the URL.
func ExampleNewClient_invalidURL() {
	_, err := redisconn.NewClient(redisconn.Config{URL: "http://localhost:6379"})
	fmt.Println(err != nil)
	// Output: true
}
