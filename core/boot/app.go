package boot

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/TBXark/sphere/core/task"
)

type Application struct {
	tasks     []task.Task
	manager   *task.Manager
	isRunning atomic.Bool
}

func NewApplication(tasks ...task.Task) *Application {
	return &Application{
		tasks: tasks,
	}
}

func (a *Application) Identifier() string {
	return "application"
}

func (a *Application) Start(ctx context.Context) error {
	if !a.isRunning.CompareAndSwap(false, true) {
		return fmt.Errorf("application is already running")
	}
	a.manager, ctx = task.NewManagerWithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i, act := range a.tasks {
		err := a.manager.StartTask(
			ctx,
			fmt.Sprintf("%d:%s", i, act.Identifier()),
			act,
		)
		if err != nil {
			return err
		}
	}
	return a.manager.Wait()
}

func (a *Application) Stop(ctx context.Context) error {
	if !a.isRunning.CompareAndSwap(true, false) {
		return fmt.Errorf("application is not running")
	}
	return a.manager.StopAll(ctx)
}
