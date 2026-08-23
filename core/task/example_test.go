package task_test

import (
	"context"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/scripttask"
)

// One-shot recipe: Start returns after every job has run and been stopped.
func ExampleNewGroup() {
	job := scripttask.NewScriptTask("migrate", func(context.Context) error {
		return nil
	}, nil)
	_ = task.NewGroup(job).Start(context.Background())
}

// Process recipe when drain order matters. Infra starts first and stops last;
// HTTP starts last and stops first. Pass the group to boot.NewApplication.
func ExampleNewStagedGroup() {
	infra := scripttask.NewScriptTask("cache-trim", func(context.Context) error {
		return nil
	}, nil)
	httpSrv := scripttask.NewScriptTask("http", nil, nil)

	_ = task.NewStagedGroup(
		[]task.Task{infra},
		[]task.Task{httpSrv},
	)
}
