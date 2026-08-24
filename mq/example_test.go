package mq_test

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/mq/memory"
)

func Example() {
	ps := memory.NewPubSub[string]()
	ctx, cancel := context.WithCancel(context.Background())
	received := make(chan string, 1)
	sub, err := ps.Subscribe(ctx, "events", func(ctx context.Context, data string) error {
		received <- data
		return nil
	})
	if err != nil {
		panic(err)
	}

	if err := ps.Broadcast(context.Background(), "events", "ready"); err != nil {
		panic(err)
	}
	fmt.Println(<-received)

	// Cancelling the subscription context requests a non-blocking stop. Done
	// reports when the handler and consumer goroutine are fully quiescent.
	cancel()
	<-sub.Done()
	if err := ps.Stop(context.Background()); err != nil {
		panic(err)
	}

	// Output: ready
}
