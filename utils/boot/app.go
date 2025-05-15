package boot

import (
	"context"
	"fmt"
)

type Task interface {
	Identifier() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Application struct {
	tasks   []Task
	manager *Manager
}

func NewApplication(tasks ...Task) *Application {
	return &Application{
		tasks:   tasks,
		manager: NewManager(),
	}
}

func (a *Application) Identifier() string {
	return "application"
}

func (a *Application) Start(ctx context.Context) error {
	for i, task := range a.tasks {
		err := a.manager.StartTask(
			ctx,
			fmt.Sprintf("%d:%s", i, task.Identifier()),
			task,
			WithStopGroupOnError(),
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
