package boot

import (
	"context"

	"github.com/go-sphere/sphere/core/task"
)

// Application is the process-level Task that Run drives. It is a thin wrapper
// around task.Group: Start and Stop forward to the group, so an Application is
// single-use like its group. Create it with NewApplication,
// NewStagedApplication, or NewApplicationFromGroup; the zero value is not
// usable.
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
// has staged waves or Group options. A nil g is replaced by an empty group.
// The Application takes over g: do not Start or Stop g directly afterwards.
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

// Identifier returns the fixed string "application".
func (a *Application) Identifier() string {
	return "application"
}

// Start begins all managed tasks in the application and reports whatever the
// underlying group reports.
//
// A graceful shutdown yields nil: the group discards the context.Canceled
// results its own teardown provokes. A task that genuinely fails during
// teardown is still reported, even if its error wraps context.Canceled.
func (a *Application) Start(ctx context.Context) error {
	return a.group.Start(ctx)
}

// Stop gracefully shuts down all managed tasks in the application.
// Returns an error if any task fails to stop cleanly.
func (a *Application) Stop(ctx context.Context) error {
	return a.group.Stop(ctx)
}
