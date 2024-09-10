package boot

import (
	"flag"
	"fmt"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/pkg/log"
	"os"
)

func DefaultCommandConfigFlagsParser() *config.Config {
	path := flag.String("config", "config.json", "config file path")
	version := flag.Bool("version", false, "show version")
	help := flag.Bool("help", false, "show help")
	flag.Parse()

	if *version {
		fmt.Println(config.BuildVersion)
		os.Exit(0)
	}

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	conf, err := config.LoadLocalConfig(*path)
	if err != nil {
		log.Panicf("load local config error: %v", err)
	}

	if conf.Remote == nil {
		return conf
	}
	conf, err = config.LoadRemoteConfig(conf.Remote.Provider, conf.Remote.Endpoint, conf.Remote.Path)
	if err != nil {
		log.Panicf("load remote config error: %v", err)
	}
	return conf

}
