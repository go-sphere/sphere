package redis_test

import (
	"context"
	"fmt"

	"github.com/alicebob/miniredis/v2"
	mqredis "github.com/go-sphere/sphere/mq/redis"
	"github.com/redis/go-redis/v9"
)

type job struct {
	Name string `json:"name"`
}

// The Redis client is owned by the caller: Stop the MessageQueue first, then
// close the client. miniredis stands in for Redis.
func ExampleNewMessageQueue() {
	ctx := context.Background()
	mini, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mini.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer func() { _ = rdb.Close() }()

	q, err := mqredis.NewMessageQueue[job](mqredis.WithClient(rdb))
	if err != nil {
		panic(err)
	}

	if err := q.Publish(ctx, "jobs", job{Name: "resize"}); err != nil {
		panic(err)
	}
	got, err := q.Consume(ctx, "jobs")
	if err != nil {
		panic(err)
	}
	fmt.Println(got.Name)

	received := make(chan job, 1)
	if _, err := q.Subscribe(ctx, "events", func(_ context.Context, v job) error {
		received <- v
		return nil
	}); err != nil {
		panic(err)
	}
	if err := q.Broadcast(ctx, "events", job{Name: "ready"}); err != nil {
		panic(err)
	}
	fmt.Println((<-received).Name)

	if err := q.Stop(ctx); err != nil {
		panic(err)
	}
	// Output:
	// resize
	// ready
}
