package scripttask_test

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/core/task/scripttask"
)

func ExampleNewScriptTask() {
	ctx := context.Background()
	st := scripttask.NewScriptTask("seed",
		func(context.Context) error {
			fmt.Println("seeding")
			return nil
		},
		func(context.Context) error {
			fmt.Println("cleanup")
			return nil
		},
	)
	if err := st.Start(ctx); err != nil {
		fmt.Println("start error:", err)
	}
	if err := st.Stop(ctx); err != nil {
		fmt.Println("stop error:", err)
	}
	fmt.Println(st.IsStarted(), st.IsStopped())
	// Output:
	// seeding
	// cleanup
	// true true
}

// A nil onStart blocks until its context ends; Stop does not cancel it.
// task.Group and task.Manager cancel the run context for you.
func ExampleScriptTask_Started() {
	ctx := context.Background()
	worker := scripttask.NewScriptTask("worker", nil, nil)

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- worker.Start(runCtx) }()

	<-worker.Started()
	cancel()
	fmt.Println("start returned:", <-done)
	if err := worker.Stop(ctx); err != nil {
		fmt.Println("stop error:", err)
	}
	fmt.Println("stopped:", worker.IsStopped())
	// Output:
	// start returned: context canceled
	// stopped: true
}
