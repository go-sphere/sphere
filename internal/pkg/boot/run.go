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
	provider := flag.String("provider", "", "config provider")
	endpoint := flag.String("endpoint", "", "config endpoint")
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

	if *provider == "" {
		conf, err := config.LoadLocalConfig(*path)
		if err != nil {
			log.Panicf("load config error: %v", err)
		}
		return conf
	}
	conf, err := config.LoadRemoteConfig(*provider, *endpoint, *path)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}
	return conf
}
