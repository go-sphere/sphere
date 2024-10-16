package app

import (
	"github.com/tbxark/sphere/internal/biz/bot"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func CreateApplication(app *bot.Bot) *boot.Application {
	return boot.NewApplication(
		[]boot.Task{
			app,
		},
		[]boot.Cleaner{})
}
