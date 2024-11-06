package boot

import (
	"context"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"golang.org/x/sync/errgroup"
)

type Runnable interface {
	Identifier() string
	Run() error
}

type Closeable interface {
	Identifier() string
	Close(ctx context.Context) error
}

type Application struct {
	runner []Runnable
	closer []Closeable
}

func NewApplication(tasks []Runnable, cleaners []Closeable) *Application {
	return &Application{
		runner: tasks,
		closer: cleaners,
	}
}

func (a *Application) Run(ctx context.Context) error {
	wg, errCtx := errgroup.WithContext(ctx)
	for _, item := range a.runner {
		log.Infof("runner %s start", item.Identifier())
		runner := item
		wg.Go(func() error {
			done := make(chan struct{})
			defer close(done)
			go func() {
				select {
				case <-errCtx.Done():
					log.Infof("runner %s stopping due to context cancellation", runner.Identifier())
					if stoppable, ok := runner.(Closeable); ok {
						_ = stoppable.Close(context.Background())
					}
				case <-done:
					// runner finished and closed this goroutine
					return
				}
			}()
			defer func() {
				if r := recover(); r != nil {
					log.Errorw(
						"runner panic",
						logfields.String("runner", runner.Identifier()),
					)
				}
			}()
			if err := runner.Run(); err != nil {
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
	for _, item := range a.closer {
		log.Infof("closer %s start", item)
		closer := item
		wg.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					log.Errorw(
						"closer panic",
						logfields.String("closer", closer.Identifier()),
					)
				}
			}()
			if err := closer.Close(ctx); err != nil {
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
