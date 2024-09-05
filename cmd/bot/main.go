package main

import (
	"github.com/tbxark/go-base-api/cmd/bot/app"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, nil, app.NewBotApplication)
	if err != nil {
		panic(err)
	}
}
