package asynq_test

import (
	"context"
	"fmt"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-sphere/sphere/scheduler/asynq"
	sasynq "github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Register handlers, run the scheduler as a task, enqueue work, then Stop
// before closing the caller-owned Redis client. miniredis stands in for Redis.
func Example() {
	mini, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mini.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer func() { _ = rdb.Close() }()

	s, err := asynq.NewScheduler(asynq.Config{Concurrency: 1},
		asynq.WithClient(rdb),
		asynq.WithLogLevel(sasynq.ErrorLevel),
	)
	if err != nil {
		panic(err)
	}
	got := make(chan string, 1)
	if err := s.Handle("email.welcome", func(_ context.Context, payload []byte) error {
		got <- string(payload)
		return nil
	}); err != nil {
		panic(err)
	}

	started := make(chan error, 1)
	go func() { started <- s.Start(context.Background()) }()

	if _, err := s.Enqueue(context.Background(), "email.welcome", []byte("hello")); err != nil {
		panic(err)
	}
	fmt.Println(<-got)

	// Stop returns nil once asynq has drained; only then close Redis.
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println(<-started)
	// Output:
	// hello
	// context canceled
}
