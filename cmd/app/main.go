package main

import (
	"github.com/tbxark/sphere/cmd/app/app"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, app.NewApplication)
	if err != nil {
		panic(err)
	}
}
