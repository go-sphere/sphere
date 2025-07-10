package boot

import (
	"context"
	"errors"

	"github.com/TBXark/sphere/core/task"
)

type Application struct {
	group *task.Group
}

func NewApplication(tasks ...task.Task) *Application {
	return &Application{
		group: task.NewGroup(tasks...),
	}
}

func (a *Application) Identifier() string {
	return "application"
}

func (a *Application) Start(ctx context.Context) error {
	err := a.group.Start(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	return nil
}

func (a *Application) Stop(ctx context.Context) error {
	return a.group.Stop(ctx)
}
