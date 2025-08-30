package telegram

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Config defines the configuration parameters for the Telegram bot.
type Config struct {
	Token string `json:"token" yaml:"token"`
}

// Bot represents a Telegram bot application with routing and middleware support.
// It wraps the underlying bot client and provides additional functionality for
// handling messages, authentication, and error processing.
type Bot struct {
	config *Config
	bot    *bot.Bot

	middlewares    []MiddlewareFunc
	noRouteHandler bot.HandlerFunc
	errorHandler   ErrorHandlerFunc
	authExtractor  AuthExtractorFunc
}

// NewApp creates a new Telegram bot application with the provided configuration and options.
// It initializes the bot client, applies middleware, and sets up default handlers.
// Returns an error if the bot token is invalid or client initialization fails.
func NewApp(config *Config, opts ...Option) (*Bot, error) {
	opt := newOptions(opts...)
	app := &Bot{
		config:         config,
		middlewares:    opt.middlewares,
		noRouteHandler: opt.noRouteHandler,
		errorHandler:   opt.errorHandler,
		authExtractor:  opt.authExtractor,
	}
	opt.botOptions = append(opt.botOptions,
		bot.WithDefaultHandler(
			func(ctx context.Context, bot *bot.Bot, update *models.Update) {
				app.noRouteHandler(ctx, bot, update)
			},
		),
	)
	app.middlewares = append(app.middlewares,
		NewAuthMiddleware(
			AuthExtractorFunc(func(ctx context.Context, update *Update) (map[string]any, error) {
				return app.authExtractor(ctx, update)
			}),
		),
	)
	client, err := bot.New(config.Token, opt.botOptions...)
	if err != nil {
		return nil, err
	}
	app.bot = client
	return app, nil
}

// Update applies additional configuration options to the underlying bot client.
func (b *Bot) Update(options ...bot.Option) {
	for _, opt := range options {
		opt(b.bot)
	}
}

// API returns the underlying Telegram bot client for direct API access.
func (b *Bot) API() *bot.Bot {
	return b.bot
}

// Start begins the bot's update polling and message processing.
// It removes any existing webhook and starts listening for updates using long polling.
func (b *Bot) Start(ctx context.Context) error {
	_, _ = b.bot.DeleteWebhook(context.Background(), &bot.DeleteWebhookParams{})
	b.bot.Start(ctx)
	return nil
}

// Close gracefully shuts down the bot and releases resources.
// It stops the update polling and closes the underlying bot client connection.
func (b *Bot) Close(ctx context.Context) error {
	_, err := b.bot.Close(ctx)
	b.bot = nil
	return err
}

// SendMessage sends a message in response to an update using the bot's client.
func (b *Bot) SendMessage(ctx context.Context, update *Update, m *Message) error {
	return SendMessage(ctx, b.bot, update, m)
}

func (b *Bot) appendMiddlewares(middlewares ...MiddlewareFunc) []MiddlewareFunc {
	mid := make([]MiddlewareFunc, 0, len(middlewares)+len(b.middlewares))
	mid = append(mid, b.middlewares...)
	mid = append(mid, middlewares...)
	return mid
}

// BindNoRoute sets the default handler for messages that don't match any specific route.
// This handler is called when no command or callback query handlers match the incoming update.
func (b *Bot) BindNoRoute(handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	b.noRouteHandler = WithMiddleware(handlerFunc, b.errorHandler, b.appendMiddlewares(middlewares...)...)
}

// BindCommand registers a handler for a specific bot command (e.g., "/start", "/help").
// The command parameter should not include the leading slash, as it will be added automatically.
func (b *Bot) BindCommand(command string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	fn := WithMiddleware(handlerFunc, b.errorHandler, b.appendMiddlewares(middlewares...)...)
	command = "/" + strings.TrimPrefix(command, "/")
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, command, bot.MatchTypePrefix, fn)
}

// BindCallback registers a handler for callback query data with a specific route prefix.
// The route is used as a prefix for matching callback query data (e.g., "menu:" matches "menu:item1").
func (b *Bot) BindCallback(route string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	fn := WithMiddleware(handlerFunc, b.errorHandler, b.appendMiddlewares(middlewares...)...)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, route+":", bot.MatchTypePrefix, fn)
}

// MessageSender defines a function that sends messages in response to updates.
type MessageSender = func(ctx context.Context, request *Update, msg *Message) error

// RouteMap maps operation names to their corresponding handler functions.
type RouteMap = map[string]func(ctx context.Context, request *Update) error

// RouteMapBuilder defines a function that builds a RouteMap from a service, codec, and message sender.
type RouteMapBuilder[S any, D any] = func(srv S, codec D, sender MessageSender) RouteMap

// BindRoute registers multiple handlers based on a route map and method metadata.
// It automatically binds commands and callback queries based on the provided operation metadata.
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
