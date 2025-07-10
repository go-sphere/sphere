package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/TBXark/sphere/log"
	"golang.org/x/sync/errgroup"
)

type Group struct {
	tasks   []Task
	started atomic.Bool
	stopped atomic.Bool
	cancel  context.CancelFunc
}

func NewGroup(tasks ...Task) *Group {
	return &Group{
		tasks: tasks,
	}
}

func (g *Group) Identifier() string {
	return "group"
}

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

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

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

func (g *Group) IsStarted() bool {
	return g.started.Load()
}

func (g *Group) IsStopped() bool {
	return g.stopped.Load()
}
