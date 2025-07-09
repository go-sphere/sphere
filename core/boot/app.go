package boot

import (
	"context"

	"github.com/TBXark/sphere/core/task"
)

type Application struct {
	group *task.Group
}

func NewApplication(tasks ...task.Task) *Application {
	return &Application{
		task.NewGroup(tasks...),
	}
}

func (a *Application) Identifier() string {
	return "application"
}

func (a *Application) Start(ctx context.Context) error {
	return a.group.Start(ctx)
}

func (a *Application) Stop(ctx context.Context) error {
	return a.group.Stop(ctx)
}
