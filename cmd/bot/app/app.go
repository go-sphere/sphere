package app

import (
	"github.com/tbxark/go-base-api/internal/biz/bot"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func CreateApplication(bot *bot.App) *boot.Application {
	return boot.NewApplication(
		[]boot.Task{
			bot,
		},
		[]boot.Cleaner{})
}
