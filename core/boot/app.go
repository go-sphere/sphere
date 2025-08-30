package boot

import (
	"context"
	"errors"

	"github.com/go-sphere/sphere/core/task"
)

// Application represents the main application container that manages a group of tasks.
// It implements the Task interface, allowing it to be composed with other components.
type Application struct {
	group *task.Group
}

// NewApplication creates a new Application instance with the given tasks.
// All provided tasks will be managed as a group with coordinated lifecycle management.
func NewApplication(tasks ...task.Task) *Application {
	return &Application{
		group: task.NewGroup(tasks...),
	}
}

// Identifier returns the application's identifier for logging and debugging.
func (a *Application) Identifier() string {
	return "application"
}

// Start begins all managed tasks in the application.
// It monitors the context for cancellation and returns nil if cancelled gracefully.
// Returns an error if any task fails to start.
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

// Stop gracefully shuts down all managed tasks in the application.
// Returns an error if any task fails to stop cleanly.
func (a *Application) Stop(ctx context.Context) error {
	return a.group.Stop(ctx)
}
