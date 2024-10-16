package telegram

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/sphere/pkg/log"
	"os"
	"os/signal"
)

type Config struct {
	Token string `json:"token"`
}

type Bot struct {
	config *Config
	bot    *bot.Bot
}

func NewApp(config *Config) *Bot {
	return &Bot{
		config: config,
	}
}

func (b *Bot) Run(options ...func(*bot.Bot) error) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithSkipGetMe(),
		bot.WithErrorsHandler(func(err error) {
			log.Errorf("bot error: %v", err)
		}),
		bot.WithDefaultHandler(func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			if update.Message != nil {
				log.Infof("receive message: %s", update.Message.Text)
			} else if update.CallbackQuery != nil {
				log.Infof("receive callback query: %s", update.CallbackQuery.Data)
			}
		}),
		bot.WithMiddlewares(NewRecoveryMiddleware()),
	}

	client, err := bot.New(b.config.Token, opts...)
	if err != nil {
		log.Panicf("create bot error: %v", err)
	}
	_, _ = client.DeleteWebhook(context.Background(), &bot.DeleteWebhookParams{})

	b.bot = client

	for _, opt := range options {
		if e := opt(client); e != nil {
			return e
		}
	}

	b.bot.Start(ctx)
	return nil
}

func (b *Bot) BindCommand(command string, handlerFunc HandlerFunc, middleware ...HandlerMiddleware) {
	fn := handlerFunc.WithMiddleware(middleware...)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, command, bot.MatchTypePrefix, func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		if e := fn(ctx, update); e != nil {
			log.Errorf("command %s error: %v", command, e)
		}
	})
}

func (b *Bot) BindCallback(route string, handlerFunc HandlerFunc, middleware ...HandlerMiddleware) {
	fn := handlerFunc.WithMiddleware(middleware...)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, route+":", bot.MatchTypePrefix, func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		if e := fn(ctx, update); e != nil {
			log.Errorf("callback %s error: %v", route, e)
		}
	})
}

func (b *Bot) SendMessage(ctx context.Context, update *models.Update, m *Message) error {
	if update.CallbackQuery != nil {
		origin := update.CallbackQuery.Message.Message
		param := m.toEditMessageTextParams(origin.Chat.ID, origin.ID)
		_, err := b.bot.EditMessageText(ctx, param)
		return err
	} else if update.Message != nil {
		param := m.toSendMessageParams(update.Message.Chat.ID)
		_, err := b.bot.SendMessage(ctx, param)
		return err
	}
	return nil
}
