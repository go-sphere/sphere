package app

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/internal/biz/api"
	"github.com/tbxark/go-base-api/internal/biz/dash"
	"github.com/tbxark/go-base-api/internal/biz/task"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
	"sync"
)

type Task interface {
	Identifier() string
	Run()
}

type Application struct {
	task []Task
}

func CreateApplication(dash *dash.Web, api *api.Web, initialize *task.Initialize) *Application {
	return &Application{
		task: []Task{
			dash,
			api,
			initialize,
		},
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
			t.Run()
		}(t)
	}
	wg.Wait()
}

func Run(conf *config.Config) error {
	log.Init(conf.Log, field.String("version", config.BuildVersion))
	defer log.Sync()
	gin.SetMode(conf.System.GinMode)
	app, err := NewApplication(conf.API, conf.Dash, conf.Database, conf.WxMini, conf.CDN)
	if err != nil {
		return err
	}
	app.Run()
	return nil
}
