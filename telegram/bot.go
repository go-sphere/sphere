package telegram

import (
	"context"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"strings"
)

type Config struct {
	Token string `json:"token" yaml:"token"`
}

type Bot struct {
	config        *Config
	bot           *bot.Bot
	ErrorHandler  ErrorHandlerFunc
	AuthExtractor AuthExtractorFunc
}

func NewApp(config *Config, options ...bot.Option) (*Bot, error) {
	if len(options) == 0 {
		options = DefaultBotOptions()
	}
	client, err := bot.New(config.Token, options...)
	if err != nil {
		return nil, err
	}
	return &Bot{
		config:        config,
		bot:           client,
		ErrorHandler:  DefaultErrorHandler,
		AuthExtractor: DefaultAuthExtractor,
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	_, _ = b.bot.DeleteWebhook(context.Background(), &bot.DeleteWebhookParams{})
	b.bot.Start(ctx)
	return nil
}

func (b *Bot) Close(ctx context.Context) error {
	_, err := b.bot.Close(ctx)
	b.bot = nil
	return err
}

func (b *Bot) ExtractorAuth(ctx context.Context, tBot *bot.Bot, update *Update) (context.Context, bool) {
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

func (b *Bot) BindCommand(command string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	fn := WithMiddleware(handlerFunc, b.ErrorHandler, middlewares...)
	command = "/" + strings.TrimPrefix(command, "/")
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, command, bot.MatchTypePrefix, func(ctx context.Context, tBot *bot.Bot, update *models.Update) {
		ctx, done := b.ExtractorAuth(ctx, tBot, (*Update)(update))
		if done {
			return
		}
		fn(ctx, tBot, update)
	})
}

func (b *Bot) BindCallback(route string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	fn := WithMiddleware(handlerFunc, b.ErrorHandler, middlewares...)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, route+":", bot.MatchTypePrefix, func(ctx context.Context, tBot *bot.Bot, update *models.Update) {
		ctx, done := b.ExtractorAuth(ctx, tBot, (*Update)(update))
		if done {
			return
		}
		fn(ctx, tBot, update)
	})
}

type MessageSender func(ctx context.Context, request *Update, msg *Message) error
type RouteMap map[string]func(ctx context.Context, request *Update) error
type RouteMapBuilder[S any, D any] func(srv S, codec D, sender MessageSender) RouteMap

func (b *Bot) BindRoute(route RouteMap, extra func(string) *MethodExtraData, operations []string, middlewares ...MiddlewareFunc) {
	for _, operation := range operations {
		info := extra(operation)
		if info.Command != "" {
			b.BindCommand(info.Command, route[operation], middlewares...)
		}
		if info.CallbackQuery != "" {
			b.BindCallback(info.CallbackQuery, route[operation], middlewares...)
		}
	}
}

func (b *Bot) Update(options ...bot.Option) {
	for _, opt := range options {
		opt(b.bot)
	}
}

func (b *Bot) API() *bot.Bot {
	return b.bot
}

func (b *Bot) SendMessage(ctx context.Context, update *Update, m *Message) error {
	return SendMessage(ctx, b.bot, update, m)
}

func SendMessage(ctx context.Context, b *bot.Bot, update *Update, m *Message) error {
	if m == nil || update == nil {
		return nil
	}
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
		bot.WithDefaultHandler(DefaultUpdateHandler),
		bot.WithErrorsHandler(func(err error) {
			log.Errorf("bot error: %v", err)
		}),
		bot.WithMiddlewares(NewRecoveryMiddleware()),
	}
}

func DefaultErrorHandler(ctx context.Context, b *bot.Bot, update *Update, err error) {
	log.Warnw("bot error", logfields.Error(err))
}

func DefaultUpdateHandler(ctx context.Context, bot *bot.Bot, update *models.Update) {
	if update.Message != nil {
		log.Infof("receive message: %s", update.Message.Text)
	}
	if update.CallbackQuery != nil {
		log.Infof("receive callback query: %s", update.CallbackQuery.Data)
	}
}

func SendErrorMessageHandler(ctx context.Context, b *bot.Bot, update *Update, err error) {
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

func DefaultAuthExtractor(ctx context.Context, update *Update) (map[string]any, error) {
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
