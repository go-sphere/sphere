package boot

import (
	"context"

	"github.com/go-sphere/sphere/core/task"
)

// Application is the process-level Task that Run drives. It is a thin wrapper
// around task.Group: Start and Stop forward to the group.
type Application struct {
	group *task.Group
}

// NewApplication groups the given tasks with task.NewGroup (concurrent start
// and stop). For ordered drain use NewStagedApplication. Close Wire-owned
// clients (sql.DB, Redis) in after-stop hooks or the injector cleanup after
// Run returns — not as a sibling Task of the HTTP server.
func NewApplication(tasks ...task.Task) *Application {
	return &Application{
		group: task.NewGroup(tasks...),
	}
}

// NewApplicationFromGroup uses g as the application's group without wrapping
// it in another NewGroup. Use this (or NewStagedApplication) when g already
// has staged waves or Group options.
func NewApplicationFromGroup(g *task.Group) *Application {
	if g == nil {
		g = task.NewGroup()
	}
	return &Application{group: g}
}

// NewStagedApplication is NewApplicationFromGroup(task.NewStagedGroup(waves...)).
func NewStagedApplication(waves ...[]task.Task) *Application {
	return NewApplicationFromGroup(task.NewStagedGroup(waves...))
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
