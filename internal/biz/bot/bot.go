package bot

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/go-base-api/pkg/log"
	"os"
	"os/signal"
)

type Config struct {
	Token string `json:"token"`
}

type App struct {
	config *Config
	bot    *bot.Bot
}

func NewApp(config *Config) *App {
	return &App{
		config: config,
	}
}

func (a *App) Identifier() string {
	return "bot"
}

func (a *App) Run() {
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
	}

	b, err := bot.New(a.config.Token, opts...)
	if err != nil {
		log.Panicf("create bot error: %v", err)
	}
	_, err = b.DeleteWebhook(context.Background(), &bot.DeleteWebhookParams{})
	if err != nil {
		log.Panicf("delete webhook error: %v", err)
	}
	a.bot = b

	a.BindCommand(CommandStart, a.HandleStart)
	a.BindCommand(CommandCounter, a.HandleCounter)
	a.BindCallback(QueryCounter, a.HandleCounter)

	a.bot.Start(ctx)
}

func (a *App) BindCommand(command string, handlerFunc bot.HandlerFunc) {
	a.bot.RegisterHandler(bot.HandlerTypeMessageText, command, bot.MatchTypeExact, handlerFunc)
}

func (a *App) BindCallback(query string, handlerFunc bot.HandlerFunc) {
	a.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, query, bot.MatchTypePrefix, handlerFunc)
}
