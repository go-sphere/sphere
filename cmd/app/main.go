package main

import (
	"github.com/tbxark/sphere/cmd/app/app"
	boot2 "github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	c := boot2.DefaultCommandConfigFlagsParser()
	err := boot2.Run(c, app.NewApplication)
	if err != nil {
		panic(err)
	}
}
