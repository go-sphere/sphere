package scheduler_test

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/scheduler"
)

// RecoverHandler turns a handler panic into an ordinary error.
func ExampleRecoverHandler() {
	h := scheduler.RecoverHandler(func(context.Context) error {
		panic("boom")
	})
	fmt.Println(h(context.Background()))
	// Output: scheduler: handler panic: boom
}

func ExampleApplyEnqueueOptions() {
	opts := scheduler.ApplyEnqueueOptions(
		scheduler.WithQueue("critical"),
		scheduler.WithMaxRetry(3),
		nil, // nil options are skipped
	)
	fmt.Println(opts.Queue, opts.MaxRetry)
	// Output: critical 3
}
