package boot

import (
	"context"
	"fmt"
	"github.com/TBXark/sphere/utils/task"
)

type Application struct {
	tasks   []task.Task
	manager *task.Manager
}

func NewApplication(tasks ...task.Task) *Application {
	return &Application{
		tasks:   tasks,
		manager: task.NewManager(),
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
			task.WithStopGroupOnError(),
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
