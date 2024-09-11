package boot

import (
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
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
						field.String("task", t.Identifier()),
					)
					wg.Done()
				}
			}()
			defer wg.Done()
			if err := t.Run(); err != nil {
				log.Errorw(
					"task error",
					field.String("task", t.Identifier()),
					field.Error(err),
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

func Run(conf *config.Config, builder func(*config.Config) (*Application, error)) error {
	log.Init(conf.Log, field.String("version", config.BuildVersion))
	app, err := builder(conf)
	if err != nil {
		return err
	}
	defer app.Clean()
	app.Run()
	return nil
}
