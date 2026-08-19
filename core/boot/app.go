package boot

import (
	"context"

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

// Start begins all managed tasks in the application and reports whatever the
// underlying group reports.
//
// A graceful shutdown still yields nil: the group already discards the
// context.Canceled results its own teardown provokes, so there is nothing here
// left to filter. What Start no longer does is blanket-swallow context.Canceled,
// which previously also masked genuine task failures that happened to occur
// while the group was tearing down. Callers mapping a non-nil result to a
// non-zero exit code should expect to see those failures surface.
func (a *Application) Start(ctx context.Context) error {
	return a.group.Start(ctx)
}

// Stop gracefully shuts down all managed tasks in the application.
// Returns an error if any task fails to stop cleanly.
func (a *Application) Stop(ctx context.Context) error {
	return a.group.Stop(ctx)
}
