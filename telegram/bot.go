package telegram

import (
	"context"
	"strings"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"github.com/go-telegram/bot"
)

type Config struct {
	Token string `json:"token" yaml:"token"`
}

type Options struct {
	botOptions []bot.Option

	embedDefaultAuthMiddleware bool
}

type Option = func(*Options)

func WithoutEmbedDefaultAuthMiddleware() Option {
	return func(o *Options) {
		o.embedDefaultAuthMiddleware = false
	}
}

func WithBotOptions(opt ...bot.Option) Option {
	return func(o *Options) {
		o.botOptions = append(o.botOptions, opt...)
	}
}

type Bot struct {
	config *Config
	bot    *bot.Bot

	middlewares []MiddlewareFunc

	noRouteHandler bot.HandlerFunc
	ErrorHandler   ErrorHandlerFunc
	AuthExtractor  AuthExtractorFunc
}

func NewApp(config *Config, opts ...Option) (*Bot, error) {
	opt := &Options{embedDefaultAuthMiddleware: true}
	for _, o := range opts {
		o(opt)
	}
	app := &Bot{
		config: config,
		noRouteHandler: func(ctx context.Context, bot *bot.Bot, update *Update) {
			if update.Message != nil {
				log.Infof("receive message: %s", update.Message.Text)
			}
			if update.CallbackQuery != nil {
				log.Infof("receive callback query: %s", update.CallbackQuery.Data)
			}
		},
		ErrorHandler: func(ctx context.Context, bot *bot.Bot, update *Update, err error) {
			log.Warnw("bot error", logfields.Error(err))
		},
	}
	if len(opt.botOptions) == 0 {
		opt.botOptions = []bot.Option{
			bot.WithSkipGetMe(),
			bot.WithMiddlewares(NewRecoveryMiddleware()),
			bot.WithDefaultHandler(app.handleNoRouteMessage),
		}
	}
	if opt.embedDefaultAuthMiddleware {
		app.middlewares = append(app.middlewares, NewAuthMiddleware(app))
	}
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

func (b *Bot) ExtractorAuth(ctx context.Context, update *Update) (map[string]any, error) {
	if b.AuthExtractor == nil {
		return nil, nil
	}
	return b.AuthExtractor(ctx, update)
}

func (b *Bot) handleNoRouteMessage(ctx context.Context, bot *bot.Bot, update *Update) {
	if b.noRouteHandler == nil {
		return
	}
	b.noRouteHandler(ctx, bot, update)
}

func (b *Bot) WithDefaultMiddlewares(middlewares []MiddlewareFunc) []MiddlewareFunc {
	mid := make([]MiddlewareFunc, 0, len(middlewares)+len(b.middlewares))
	mid = append(mid, b.middlewares...)
	mid = append(mid, middlewares...)
	return mid
}

func (b *Bot) BindNoRoute(handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	b.noRouteHandler = WithMiddleware(handlerFunc, b.ErrorHandler, b.WithDefaultMiddlewares(middlewares)...)
}

func (b *Bot) BindCommand(command string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	fn := WithMiddleware(handlerFunc, b.ErrorHandler, b.WithDefaultMiddlewares(middlewares)...)
	command = "/" + strings.TrimPrefix(command, "/")
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, command, bot.MatchTypePrefix, fn)
}

func (b *Bot) BindCallback(route string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	fn := WithMiddleware(handlerFunc, b.ErrorHandler, b.WithDefaultMiddlewares(middlewares)...)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, route+":", bot.MatchTypePrefix, fn)
}

type (
	MessageSender                 func(ctx context.Context, request *Update, msg *Message) error
	RouteMap                      map[string]func(ctx context.Context, request *Update) error
	RouteMapBuilder[S any, D any] func(srv S, codec D, sender MessageSender) RouteMap
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
