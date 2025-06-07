package boot

import (
	"context"
	"fmt"

	task2 "github.com/TBXark/sphere/core/task"
)

type Application struct {
	tasks   []task2.Task
	manager *task2.Manager
}

func NewApplication(tasks ...task2.Task) *Application {
	return &Application{
		tasks:   tasks,
		manager: task2.NewManager(),
	}
}

func (a *Application) Identifier() string {
	return "application"
}

func (a *Application) Start(ctx context.Context) error {
	for i, act := range a.tasks {
		err := a.manager.StartTask(
			ctx,
			fmt.Sprintf("%d:%s", i, act.Identifier()),
			act,
			task2.WithStopGroupOnError(),
		)
		if err != nil {
			return err
		}
	}
	return a.manager.Wait()
}

func (a *Application) Stop(ctx context.Context) error {
	return a.manager.StopAll(ctx)
}
