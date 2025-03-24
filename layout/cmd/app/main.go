package main

import (
	"flag"

	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/TBXark/sphere/layout/internal/pkg/app"
	"github.com/TBXark/sphere/utils/boot"
)

func main() {
	mode := flag.String("mode", "app", "run mode: app, api, dash, bot")
	app.Execute(func(config *config.Config) (*boot.Application, error) {
		switch *mode {
		case "api":
			return NewAPIApplication(config)
		case "dash":
			return NewDashApplication(config)
		case "bot":
			return NewBotApplication(config)
		default:
			return NewApplication(config)
		}
	})
}
