package main

import (
	"github.com/tbxark/sphere/cmd/api/app"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, app.NewAPIApplication)
	if err != nil {
		panic(err)
	}
}
