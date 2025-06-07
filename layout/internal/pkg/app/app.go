package app

import (
	"fmt"
	"os"

	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/TBXark/sphere/utils/boot"
)

func Execute(app func(*config.Config) (*boot.Application, error)) {
	conf := boot.DefaultConfigParser(config.BuildVersion, config.NewConfig)
	err := boot.Run(conf, app, boot.WithLoggerInit(config.BuildVersion, conf.Log))
	if err != nil {
		fmt.Printf("Boot error: %v", err)
		os.Exit(1)
	}
	fmt.Println("Boot done")
}
