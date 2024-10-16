package main

import (
	"github.com/tbxark/sphere/cmd/dash/app"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, app.NewDashApplication)
	if err != nil {
		panic(err)
	}
}
