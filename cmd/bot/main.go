package main

import (
	"github.com/tbxark/sphere/internal/pkg/app"
	"github.com/tbxark/sphere/internal/server/bot"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	app.Execute(NewBotApplication)
}

func newApplication(app *bot.Bot) *boot.Application {
	return boot.NewApplication(app)
}
