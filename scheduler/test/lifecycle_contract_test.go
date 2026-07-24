package test

import (
	"testing"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

func TestSchedulerLifecycleContract(t *testing.T) {
	for _, factory := range cronFactories() {
		t.Run(factory.name, func(t *testing.T) {
			tasktest.AssertLifecycleContract(t, func() task.Task {
				runtime := factory.new(t)
				tk, ok := runtime.(task.Task)
				if !ok {
					t.Fatalf("%s scheduler does not implement task.Task", factory.name)
				}
				return tk
			})
		})
	}
}
