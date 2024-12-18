package main

import (
	"github.com/TBXark/sphere/internal/pkg/app"
	"github.com/TBXark/sphere/internal/server/bot"
	"github.com/TBXark/sphere/pkg/utils/boot"
)

func main() {
	app.Execute(NewBotApplication)
}

func newApplication(app *bot.Bot) *boot.Application {
	return boot.NewApplication(app)
}
