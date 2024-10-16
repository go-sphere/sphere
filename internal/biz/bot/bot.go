package bot

import (
	"github.com/go-telegram/bot"
	"github.com/tbxark/sphere/pkg/telegram"
)

type Config telegram.Config

type Bot struct {
	*telegram.Bot
}

func NewApp(conf *Config) (*Bot, error) {
	app := telegram.NewApp((*telegram.Config)(conf))
	return &Bot{
		Bot: app,
	}, nil
}

func (b *Bot) Identifier() string {
	return "bot"
}

func (b *Bot) Run() error {
	return b.Bot.Run(func(bot *bot.Bot) error {

		sfMid := telegram.NewSingleFlightMiddleware()
		errMid := telegram.NewErrorAlertMiddleware(bot)

		b.BindCommand(CommandStart, b.HandleStart, errMid)
		b.BindCommand(CommandCounter, b.HandleCounter, errMid, sfMid)
		b.BindCallback(QueryCounter, b.HandleCounter, errMid, sfMid)

		return nil
	})
}
