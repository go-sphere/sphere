package boot

import (
	"flag"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/pkg/log"
)

type Runnable interface {
	Run() error
}

func RunWithConfig[R Runnable](name string, builder func(config *config.Config) (R, error)) error {
	cfg := flag.String("config", "config.json", "config file path")
	flag.Parse()
	conf, err := config.LoadConfig(*cfg)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}
	dash, err := builder(conf)
	if err != nil {
		log.Panicf("create %s error: %v", name, err)
	}
	return dash.Run()
}
