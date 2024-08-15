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
	_, _ = b.DeleteWebhook(context.Background(), &bot.DeleteWebhookParams{})

	a.bot = b
	a.BindCommand(CommandStart, a.HandleStart)
	a.BindCommand(CommandCounter, a.HandleCounter)
	a.BindCallback(QueryCounter, a.HandleCounter)

	a.bot.Start(ctx)
}

type HandlerFunc func(ctx context.Context, update *models.Update) error

func (a *App) BindCommand(command string, handlerFunc HandlerFunc) {
	a.bot.RegisterHandler(bot.HandlerTypeMessageText, command, bot.MatchTypeExact, func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		if e := handlerFunc(ctx, update); e != nil {
			log.Errorf("command %s error: %v", command, e)
		}
	})
}

func (a *App) BindCallback(query string, handlerFunc HandlerFunc) {
	a.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, query, bot.MatchTypePrefix, func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		if e := handlerFunc(ctx, update); e != nil {
			log.Errorf("callback %s error: %v", query, e)
		}
	})
}

func (a *App) SendMessage(ctx context.Context, update *models.Update, m *Message) error {
	if update.CallbackQuery != nil {
		origin := update.CallbackQuery.Message.Message
		_, err := a.bot.EditMessageText(ctx, m.toEditMessageTextParams(origin.Chat.ID, origin.ID))
		return err
	} else if update.Message != nil {
		_, err := a.bot.SendMessage(ctx, m.toSendMessageParams(update.Message.Chat.ID))
		return err
	}
	return nil
}

func unmarshalUpdateData[T any](update *models.Update) (*T, error) {
	if update.CallbackQuery != nil {
		return unmarshalData[T](update.CallbackQuery.Data)
	}
	return nil, nil
}

func unmarshalUpdateDataX[T any](update *models.Update, t T) (*T, error) {
	if update.CallbackQuery != nil {
		return unmarshalData[T](update.CallbackQuery.Data)
	}
	return &t, nil
}
