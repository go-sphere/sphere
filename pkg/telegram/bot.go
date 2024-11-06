package telegram

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/sphere/pkg/log"
)

type Config struct {
	Token string `json:"token" yaml:"token"`
}

type Bot struct {
	config        *Config
	bot           *bot.Bot
	BotOptions    func() []bot.Option
	ErrorHandler  ErrorHandlerFunc
	AuthExtractor AuthExtractorFunc
}

func NewApp(config *Config) *Bot {
	return &Bot{
		config:        config,
		BotOptions:    DefaultBotOptions,
		ErrorHandler:  DefaultErrorHandler,
		AuthExtractor: DefaultAuthExtractor,
	}
}

func (b *Bot) Run(ctx context.Context, options ...func(*bot.Bot) error) error {
	client, err := bot.New(b.config.Token, b.BotOptions()...)
	if err != nil {
		log.Panicf("create bot error: %v", err)
	}
	b.bot = client
	for _, opt := range options {
		if e := opt(client); e != nil {
			return e
		}
	}
	_, _ = client.DeleteWebhook(context.Background(), &bot.DeleteWebhookParams{})
	b.bot.Start(ctx)
	return nil
}

func (b *Bot) Close(ctx context.Context) error {
	if b.bot == nil {
		return nil
	}
	_, err := b.bot.Close(ctx)
	b.bot = nil
	return err
}

func (b *Bot) ExtractorAuth(ctx context.Context, tBot *bot.Bot, update *models.Update) (context.Context, bool) {
	if b.AuthExtractor != nil {
		info, err := b.AuthExtractor(ctx, update)
		if err != nil {
			if b.ErrorHandler != nil {
				b.ErrorHandler(ctx, tBot, update, err)
			}
			return nil, true
		}
		c := NewContext(ctx)
		for k, v := range info {
			c.SetValue(k, v)
		}
		return c, false
	}
	return ctx, false
}

func (b *Bot) BindCommand(command string, handlerFunc HandlerFunc, middleware ...bot.Middleware) {
	fn := WithMiddleware(handlerFunc, b.ErrorHandler, middleware...)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, command, bot.MatchTypePrefix, func(ctx context.Context, tBot *bot.Bot, update *models.Update) {
		ctx, done := b.ExtractorAuth(ctx, tBot, update)
		if done {
			return
		}
		fn(ctx, tBot, update)
	})
}

func (b *Bot) BindCallback(route string, handlerFunc HandlerFunc, middleware ...bot.Middleware) {
	fn := WithMiddleware(handlerFunc, b.ErrorHandler, middleware...)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, route+":", bot.MatchTypePrefix, func(ctx context.Context, tBot *bot.Bot, update *models.Update) {
		ctx, done := b.ExtractorAuth(ctx, tBot, update)
		if done {
			return
		}
		fn(ctx, tBot, update)
	})
}

func (b *Bot) SendMessage(ctx context.Context, update *models.Update, m *Message) error {
	return SendMessage(ctx, b.bot, update, m)
}

func SendMessage(ctx context.Context, b *bot.Bot, update *models.Update, m *Message) error {
	if update.CallbackQuery != nil {
		origin := update.CallbackQuery.Message.Message
		param := m.toEditMessageTextParams(origin.Chat.ID, origin.ID)
		_, err := b.EditMessageText(ctx, param)
		return err
	} else if update.Message != nil {
		param := m.toSendMessageParams(update.Message.Chat.ID)
		_, err := b.SendMessage(ctx, param)
		return err
	}
	return nil
}

func DefaultBotOptions() []bot.Option {
	return []bot.Option{
		bot.WithSkipGetMe(),
		bot.WithErrorsHandler(func(err error) {
			log.Errorf("bot error: %v", err)
		}),
		bot.WithDefaultHandler(func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			if update.Message != nil {
				log.Infof("receive message: %s", update.Message.Text)
			}
			if update.CallbackQuery != nil {
				log.Infof("receive callback query: %s", update.CallbackQuery.Data)
			}
		}),
		bot.WithMiddlewares(NewRecoveryMiddleware()),
	}
}

func DefaultErrorHandler(ctx context.Context, b *bot.Bot, update *models.Update, err error) {
	if err == nil {
		return
	}
	if update.Message != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   err.Error(),
		})
	}
	if update.CallbackQuery != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            err.Error(),
		})
	}
}

func DefaultAuthExtractor(ctx context.Context, update *models.Update) (map[string]any, error) {
	var user *models.User
	if update.Message != nil {
		user = update.Message.From
	}
	if update.CallbackQuery != nil {
		user = &update.CallbackQuery.From
	}
	if user == nil {
		return nil, nil
	}
	return map[string]any{
		"uid":     user.ID,
		"subject": user.Username,
	}, nil
}
