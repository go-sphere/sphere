package telegram

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Config struct {
	Token string `json:"token" yaml:"token"`
}

type Bot struct {
	config *Config
	bot    *bot.Bot

	middlewares    []MiddlewareFunc
	noRouteHandler bot.HandlerFunc
	errorHandler   ErrorHandlerFunc
	authExtractor  AuthExtractorFunc
}

func NewApp(config *Config, opts ...Option) (*Bot, error) {
	opt := newOptions(opts...)
	app := &Bot{
		config:         config,
		middlewares:    opt.middlewares,
		noRouteHandler: opt.noRouteHandler,
		errorHandler:   opt.errorHandler,
		authExtractor:  opt.authExtractor,
	}
	opt.botOptions = append(opt.botOptions, bot.WithDefaultHandler(
		func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			app.noRouteHandler(ctx, bot, update)
		},
	))
	client, err := bot.New(config.Token, opt.botOptions...)
	if err != nil {
		return nil, err
	}
	app.bot = client
	return app, nil
}

func (b *Bot) Update(options ...bot.Option) {
	for _, opt := range options {
		opt(b.bot)
	}
}

func (b *Bot) API() *bot.Bot {
	return b.bot
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

func (b *Bot) SendMessage(ctx context.Context, update *Update, m *Message) error {
	return SendMessage(ctx, b.bot, update, m)
}

func (b *Bot) withMiddlewares(middlewares ...MiddlewareFunc) []MiddlewareFunc {
	mid := make([]MiddlewareFunc, 0, len(middlewares)+len(b.middlewares))
	mid = append(mid, b.middlewares...)
	mid = append(mid, middlewares...)
	return mid
}

func (b *Bot) BindNoRoute(handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	b.noRouteHandler = WithMiddleware(handlerFunc, b.errorHandler, b.withMiddlewares(middlewares...)...)
}

func (b *Bot) BindCommand(command string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	fn := WithMiddleware(handlerFunc, b.errorHandler, b.withMiddlewares(middlewares...)...)
	command = "/" + strings.TrimPrefix(command, "/")
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, command, bot.MatchTypePrefix, fn)
}

func (b *Bot) BindCallback(route string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	fn := WithMiddleware(handlerFunc, b.errorHandler, b.withMiddlewares(middlewares...)...)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, route+":", bot.MatchTypePrefix, fn)
}

type (
	MessageSender                 = func(ctx context.Context, request *Update, msg *Message) error
	RouteMap                      = map[string]func(ctx context.Context, request *Update) error
	RouteMapBuilder[S any, D any] = func(srv S, codec D, sender MessageSender) RouteMap
)

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
