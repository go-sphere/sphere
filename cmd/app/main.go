package main

import (
	"github.com/tbxark/sphere/cmd/app/app"
	"github.com/tbxark/sphere/config"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	conf := boot.DefaultCommandConfigFlagsParser(config.NewConfig)
	err := boot.Run(conf, app.NewApplication)
	if err != nil {
		panic(err)
	}
}
