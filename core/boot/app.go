package boot

import (
	"context"
	"errors"
	"github.com/TBXark/sphere/core/task"
	"golang.org/x/sync/errgroup"
	"sync"
	"sync/atomic"
)

type Application struct {
	identifier string
	tasks      []task.Task
	started    atomic.Bool
	stopped    atomic.Bool
	cancel     context.CancelFunc
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
	if !a.started.CompareAndSwap(false, true) {
		return errors.New("task group already started")
	}

	if a.stopped.Load() {
		return errors.New("task group already stopped")
	}

	groupCtx, groupCancel := context.WithCancel(ctx)
	a.cancel = groupCancel

	eg, egCtx := errgroup.WithContext(groupCtx)
	wg := sync.WaitGroup{}

	for _, tt := range a.tasks {
		t := tt
		eg.Go(func() error {
			<-egCtx.Done()
			return t.Stop(ctx)
		})
		wg.Add(1)
		eg.Go(func() error {
			wg.Done()
			return t.Start(egCtx)
		})
	}

	wg.Wait()

	eg.Go(func() error {
		select {
		case <-egCtx.Done():
			return egCtx.Err()
		case <-ctx.Done():
			return a.Stop(ctx)
		}
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func (a *Application) Stop(ctx context.Context) error {
	if !a.stopped.CompareAndSwap(false, true) {
		return nil
	}
	if !a.started.Load() {
		return errors.New("task group not started")
	}
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

func (a *Application) IsStarted() bool {
	return a.started.Load()
}

func (a *Application) IsStopped() bool {
	return a.stopped.Load()
}
