package main

import (
	"github.com/tbxark/sphere/cmd/bot/app"
	boot2 "github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	c := boot2.DefaultCommandConfigFlagsParser()
	err := boot2.Run(c, app.NewBotApplication)
	if err != nil {
		panic(err)
	}
}
