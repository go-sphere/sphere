package boot

import (
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"sync"
)

type Task interface {
	Identifier() string
	Run() error
}

type Cleaner interface {
	Clean() error
}

type Application struct {
	task    []Task
	cleaner []Cleaner
}

func NewApplication(tasks []Task, cleaners []Cleaner) *Application {
	return &Application{
		task:    tasks,
		cleaner: cleaners,
	}
}

func (a *Application) Run() {
	wg := sync.WaitGroup{}
	for _, t := range a.task {
		wg.Add(1)

		log.Infof("task %s start", t.Identifier())
		go func(t Task) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorw(
						"task panic",
						logfields.String("task", t.Identifier()),
					)
					wg.Done()
				}
			}()
			defer wg.Done()
			if err := t.Run(); err != nil {
				log.Errorw(
					"task error",
					logfields.String("task", t.Identifier()),
					logfields.Error(err),
				)
			}
		}(t)
	}
	wg.Wait()
}

func (a *Application) Clean() {
	for _, c := range a.cleaner {
		_ = c.Clean()
	}
}
