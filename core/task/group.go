package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/go-sphere/sphere/log"
	"golang.org/x/sync/errgroup"
)

// Group manages the lifecycle of multiple tasks as a coordinated unit.
// It ensures all tasks start together and provides graceful shutdown capabilities.
// The group implements the Task interface, allowing it to be nested within other groups.
type Group struct {
	tasks   []Task
	started atomic.Bool
	stopped atomic.Bool
	cancel  context.CancelFunc
}

// NewGroup creates a new task group with the provided tasks.
// All tasks will be managed together with coordinated startup and shutdown.
func NewGroup(tasks ...Task) *Group {
	return &Group{
		tasks: tasks,
	}
}

// Identifier returns the group's identifier for logging and debugging purposes.
func (g *Group) Identifier() string {
	return "group"
}

// Start begins all tasks in the group concurrently.
// If any task fails to start, all other tasks will be stopped.
// Returns an error if the group is already started/stopped or if any task fails.
func (g *Group) Start(ctx context.Context) error {
	if !g.started.CompareAndSwap(false, true) {
		return errors.New("task group already started")
	}

	if g.stopped.Load() {
		return errors.New("task group already stopped")
	}

	groupCtx, groupCancel := context.WithCancel(ctx)
	g.cancel = groupCancel

	eg, egCtx := errgroup.WithContext(groupCtx)
	wg := sync.WaitGroup{}

	for _, tt := range g.tasks {
		t := tt
		eg.Go(func() error {
			<-egCtx.Done()
			return execute(ctx, t.Identifier(), t, func(ctx context.Context, task Task) error {
				log.Infof("<task> %s stopping", t.Identifier())
				return task.Stop(ctx)
			})
		})
		wg.Add(1)
		eg.Go(func() error {
			wg.Done()
			return execute(egCtx, t.Identifier(), t, func(ctx context.Context, task Task) error {
				log.Infof("<task> %s starting", t.Identifier())
				return task.Start(ctx)
			})
		})
	}

	wg.Wait()

	eg.Go(func() error {
		select {
		case <-egCtx.Done():
			return egCtx.Err()
		case <-ctx.Done():
			return g.Stop(ctx)
		}
	})

	err := eg.Wait()
	if err != nil {
		return err
	}

	return nil
}

// Stop gracefully shuts down all tasks in the group.
// It cancels the group context, triggering shutdown of all managed tasks.
// Returns an error if the group was not started, or nil if already stopped.
func (g *Group) Stop(ctx context.Context) error {
	if !g.stopped.CompareAndSwap(false, true) {
		return nil
	}
	if !g.started.Load() {
		return errors.New("task group not started")
	}
	if g.cancel != nil {
		g.cancel()
	}
	return nil
}

// IsStarted returns whether the group has been started.
func (g *Group) IsStarted() bool {
	return g.started.Load()
}

// IsStopped returns whether the group has been stopped.
func (g *Group) IsStopped() bool {
	return g.stopped.Load()
}
