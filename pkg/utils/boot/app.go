package boot

import (
	"context"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"golang.org/x/sync/errgroup"
)

type Task interface {
	Identifier() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Application struct {
	tasks []Task
}

func NewApplication(tasks ...Task) *Application {
	return &Application{
		tasks: tasks,
	}
}

func (a *Application) Run(ctx context.Context) error {
	wg, ctx := errgroup.WithContext(ctx)
	for _, task := range a.tasks {
		log.Infof("runner %s start", task.Identifier())
		runner := task
		wg.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					log.Errorw(
						"runner panic",
						logfields.String("runner", runner.Identifier()),
						logfields.Any("recover", r),
					)
				}
			}()
			if err := runner.Start(ctx); err != nil {
				log.Errorw(
					"runner error",
					logfields.String("runner", runner.Identifier()),
					logfields.Error(err),
				)
				return err
			}
			return nil
		})
	}
	return wg.Wait()
}

func (a *Application) Close(ctx context.Context) error {
	wg := errgroup.Group{}
	for _, task := range a.tasks {
		log.Infof("closer %s start", task.Identifier())
		closer := task
		wg.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					log.Errorw(
						"closer panic",
						logfields.String("closer", closer.Identifier()),
						logfields.Any("recover", r),
					)
				}
			}()
			if err := closer.Stop(ctx); err != nil {
				log.Errorw(
					"closer error",
					logfields.String("closer", closer.Identifier()),
					logfields.Error(err),
				)
				return err
			}
			return nil
		})
	}
	return wg.Wait()
}
