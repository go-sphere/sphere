package bot

import (
	"context"
	botv1 "github.com/TBXark/sphere/api/bot/v1"
	service "github.com/TBXark/sphere/internal/service/bot"
	"github.com/TBXark/sphere/pkg/telegram"
	"github.com/go-telegram/bot"
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
	route := botv1.RegisterCounterServiceBotServer(b.service, &CounterServiceCodec{}, b.SendMessage)
	b.BindCommand(botv1.ExtraBotDataCounterServiceStart.Command, route[botv1.OperationBotCounterServiceStart])
	b.BindCommand(botv1.ExtraBotDataCounterServiceCounter.Command, route[botv1.OperationBotCounterServiceCounter], sfMid)
	b.BindCallback(botv1.ExtraBotDataCounterServiceCounter.CallbackQuery, route[botv1.OperationBotCounterServiceCounter], sfMid)
	return nil
}

func (b *Bot) Start(ctx context.Context) error {
	return b.Bot.Run(ctx, b.initBot)
}

func (b *Bot) Stop(ctx context.Context) error {
	return b.Bot.Close(ctx)
}

func NewButton[T any](text, query string, data T) telegram.Button {
	return telegram.NewButton(text, query, data)
}

func UnmarshalUpdateDataWithDefault[T any](update *telegram.Update, defaultValue T) T {
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
