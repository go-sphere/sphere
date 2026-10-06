package tasktest_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/core/task/tasktest"
)

// A ModeServer fake ignores ctx and only returns from Start after Stop, like
// an HTTP listener.
func ExampleNewFake() {
	ctx := context.Background()
	f := tasktest.NewFake("http")
	f.Mode = tasktest.ModeServer
	f.StopErr = errors.New("listener close failed")

	done := make(chan error, 1)
	go func() { done <- f.Start(ctx) }()
	<-f.Started()

	fmt.Println("stop:", f.Stop(ctx))
	fmt.Println("start:", <-done)
	fmt.Println("stop again:", f.Stop(ctx))
	fmt.Println("starts:", f.StartCount(), "stops:", f.StopCount())
	// Output:
	// stop: listener close failed
	// start: <nil>
	// stop again: listener close failed
	// starts: 1 stops: 2
}
