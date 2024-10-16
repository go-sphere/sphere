package main

import (
	"github.com/tbxark/sphere/cmd/bot/app"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, app.NewBotApplication)
	if err != nil {
		panic(err)
	}
}
