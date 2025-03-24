package bot

import (
	"context"

	botv1 "github.com/TBXark/sphere/layout/api/bot/v1"
	service "github.com/TBXark/sphere/layout/internal/service/bot"
	"github.com/TBXark/sphere/telegram"
)

type Config telegram.Config

type Bot struct {
	bot     *telegram.Bot
	service *service.Service
}

func NewApp(conf *Config, botService *service.Service) (*Bot, error) {
	app, err := telegram.NewApp((*telegram.Config)(conf))
	if err != nil {
		return nil, err
	}
	return &Bot{
		bot:     app,
		service: botService,
	}, nil
}

func (b *Bot) Identifier() string {
	return "bot"
}

func (b *Bot) Start(ctx context.Context) error {
	b.bot.BindRoute(
		botv1.RegisterMenuServiceBotServer(b.service, &MenuServiceBotCodec{}, b.bot.SendMessage),
		botv1.GetExtraBotDataByMenuServiceOperation,
		botv1.GetAllBotMenuServiceOperations(),
	)
	return b.bot.Start(ctx)
}

func (b *Bot) Stop(ctx context.Context) error {
	return b.bot.Close(ctx)
}
