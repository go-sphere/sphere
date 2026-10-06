package memory_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/mq/memory"
)

// Close stops new publishes, but messages already accepted are still
// delivered before Consume reports ErrQueueClosed.
func ExampleNewQueue() {
	ctx := context.Background()
	q := memory.NewQueue[string](memory.WithQueueSize(16))

	if err := q.Publish(ctx, "jobs", "resize"); err != nil {
		panic(err)
	}
	if err := q.Close(); err != nil {
		panic(err)
	}

	job, err := q.Consume(ctx, "jobs")
	fmt.Println(job, err)
	_, err = q.Consume(ctx, "jobs")
	fmt.Println(errors.Is(err, memory.ErrQueueClosed))
	// Output:
	// resize <nil>
	// true
}

// TryConsume reports an empty queue as ok=false with a nil error. Check the
// error first.
func ExampleQueue_TryConsume() {
	ctx := context.Background()
	q := memory.NewQueue[int]()
	defer func() { _ = q.Close() }()

	_, ok, err := q.TryConsume(ctx, "jobs")
	if err != nil {
		panic(err)
	}
	fmt.Println(ok)
	// Output: false
}

func ExampleNewMessageQueue() {
	ctx := context.Background()
	mq := memory.NewMessageQueue[string](memory.WithIdentifier("events"))

	if err := mq.Publish(ctx, "jobs", "resize"); err != nil {
		panic(err)
	}
	job, err := mq.Consume(ctx, "jobs")
	if err != nil {
		panic(err)
	}
	fmt.Println(job)

	received := make(chan string, 1)
	if _, err := mq.Subscribe(ctx, "events", func(_ context.Context, v string) error {
		received <- v
		return nil
	}); err != nil {
		panic(err)
	}
	if err := mq.Broadcast(ctx, "events", "ready"); err != nil {
		panic(err)
	}
	fmt.Println(<-received)

	// Stop closes the queue and waits for every PubSub handler to return.
	if err := mq.Stop(ctx); err != nil {
		panic(err)
	}
	// Output:
	// resize
	// ready
}
