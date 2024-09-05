package boot

import (
	"flag"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/pkg/log"
)

func DefaultCommandConfigFlagsParser() *config.Config {
	cfg := flag.String("config", "config.json", "config file path")
	flag.Parse()
	conf, err := config.LoadConfig(*cfg)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}
	return conf
}
