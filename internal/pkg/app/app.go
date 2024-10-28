package app

import (
	"fmt"
	"github.com/tbxark/sphere/internal/config"
	"github.com/tbxark/sphere/pkg/utils/boot"
	"os"
)

func Execute(app func(*config.Config) (*boot.Application, error)) {
	conf := boot.DefaultConfigParser(config.BuildVersion, config.NewConfig)
	err := boot.Run(config.BuildVersion, conf, conf.Log, app)
	if err != nil {
		fmt.Printf("boot error: %v", err)
		os.Exit(1)
	}
}
