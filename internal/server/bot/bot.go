package bot

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	botv1 "github.com/tbxark/sphere/api/bot/v1"
	botSrv "github.com/tbxark/sphere/internal/service/bot"
	"github.com/tbxark/sphere/pkg/telegram"
)

type Config telegram.Config

type Bot struct {
	*telegram.Bot
	botSrv *botSrv.Service
}

func NewApp(conf *Config, botService *botSrv.Service) *Bot {
	app := telegram.NewApp((*telegram.Config)(conf))
	return &Bot{
		Bot:    app,
		botSrv: botService,
	}
}

func (b *Bot) Identifier() string {
	return "bot"
}

func (b *Bot) Run(ctx context.Context) error {
	return b.Bot.Run(ctx, func(bot *bot.Bot) error {

		sfMid := telegram.NewSingleFlightMiddleware()

		route := botv1.RegisterBotServiceBotServer(b.botSrv, b)
		b.BindCommand(CommandStart, route[botv1.BotHandlerBotServiceStart])
		b.BindCommand(CommandCounter, route[botv1.BotHandlerBotServiceCounter], sfMid)
		b.BindCallback(QueryCounter, route[botv1.BotHandlerBotServiceCounter], sfMid)

		return nil
	})
}

func (b *Bot) Close(ctx context.Context) error {
	return b.Bot.Close(ctx)
}

func UnmarshalUpdateDataWithDefault[T any](update *models.Update, defaultValue T) T {
	if update != nil && update.CallbackQuery != nil {
		_, data, err := telegram.UnmarshalData[T](update.CallbackQuery.Data)
		if err == nil {
			return *data
		}
		return defaultValue
	} else {
		return defaultValue
	}
}
