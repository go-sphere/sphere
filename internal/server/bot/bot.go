package bot

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	botv1 "github.com/tbxark/sphere/api/bot/v1"
	service "github.com/tbxark/sphere/internal/service/bot"
	"github.com/tbxark/sphere/pkg/telegram"
)

type Config telegram.Config

type Bot struct {
	*telegram.Bot
	service *service.Service
}

func NewApp(conf *Config, botService *service.Service) *Bot {
	app := telegram.NewApp((*telegram.Config)(conf))
	return &Bot{
		Bot:     app,
		service: botService,
	}
}

func (b *Bot) Identifier() string {
	return "bot"
}

func (b *Bot) initBot(t *bot.Bot) error {
	sfMid := telegram.NewSingleFlightMiddleware()
	route := botv1.RegisterCounterServiceBotServer(b.service, &CounterServiceCodec{}, telegram.SendMessage)
	b.BindCommand(CommandStart, route[botv1.BotHandlerCounterServiceStart])
	b.BindCommand(CommandCounter, route[botv1.BotHandlerCounterServiceCounter], sfMid)
	b.BindCallback(QueryCounter, route[botv1.BotHandlerCounterServiceCounter], sfMid)
	return nil
}

func (b *Bot) Run(ctx context.Context) error {
	return b.Bot.Run(ctx, b.initBot)
}

func (b *Bot) Close(ctx context.Context) error {
	return b.Bot.Close(ctx)
}

func NewButton[T any](text, query string, data T) telegram.Button {
	return telegram.NewButton(text, query, data)
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
