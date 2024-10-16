package app

import (
	"github.com/tbxark/go-base-api/internal/biz/bot"
	"github.com/tbxark/go-base-api/pkg/utils/boot"
)

func CreateApplication(app *bot.Bot) *boot.Application {
	return boot.NewApplication(
		[]boot.Task{
			app,
		},
		[]boot.Cleaner{})
}
