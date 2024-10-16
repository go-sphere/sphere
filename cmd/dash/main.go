package main

import (
	"github.com/tbxark/sphere/cmd/dash/app"
	boot2 "github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	c := boot2.DefaultCommandConfigFlagsParser()
	err := boot2.Run(c, app.NewDashApplication)
	if err != nil {
		panic(err)
	}
}
