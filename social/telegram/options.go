package telegram

import (
	"context"

	"github.com/go-sphere/sphere/log"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// options holds configuration options for creating a Telegram bot application.
type options struct {
	noRouteHandler bot.HandlerFunc   // Handler for unmatched routes
	errorHandler   ErrorHandlerFunc  // Handler for processing errors
	authExtractor  AuthExtractorFunc // Function to extract authentication data

	botOptions  []bot.Option     // Options to pass to the underlying bot client
	middlewares []MiddlewareFunc // Middleware functions to apply to handlers
}

// Option defines a function type for configuring bot application options.
type Option = func(*options)

func newOptions(opts ...Option) *options {
	defaults := &options{
		noRouteHandler: func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			if update.Message != nil {
				log.Infof("receive message: %s", update.Message.Text)
			}
			if update.CallbackQuery != nil {
				log.Infof("receive callback query: %s", update.CallbackQuery.Data)
			}
		},
		errorHandler: func(ctx context.Context, bot *bot.Bot, update *Update, err error) {
			log.Warn("bot error", log.Err(err))
		},
		authExtractor: DefaultAuthExtractor,
		botOptions: []bot.Option{
			bot.WithSkipGetMe(),
			bot.WithMiddlewares(NewRecoveryMiddleware()),
		},
		middlewares: []MiddlewareFunc{},
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// WithErrorHandler sets a custom error handler for bot operations.
// The error handler will be called whenever a handler function returns an error.
func WithErrorHandler(fn ErrorHandlerFunc) Option {
	return func(o *options) {
		o.errorHandler = fn
	}
}

// WithDefaultHandler sets a custom handler for unmatched routes.
// This handler will be called when no specific command or callback query handler matches.
func WithDefaultHandler(fn bot.HandlerFunc) Option {
	return func(o *options) {
		o.noRouteHandler = fn
	}
}

// WithAuthExtractor sets a custom authentication extractor for the bot.
// The extractor will be used to extract user information from incoming updates.
func WithAuthExtractor(extractor AuthExtractorFunc) Option {
	return func(o *options) {
		o.authExtractor = extractor
	}
}

// AppendBotOptions adds additional options to the underlying bot client configuration.
// These options will be passed directly to the bot.New() constructor.
func AppendBotOptions(opt ...bot.Option) Option {
	return func(o *options) {
		o.botOptions = append(o.botOptions, opt...)
	}
}

// AppendMiddlewares adds middleware functions to the bot application.
// Middleware will be applied to all handlers in the order they are provided.
func AppendMiddlewares(middlewares ...MiddlewareFunc) Option {
	return func(o *options) {
		o.middlewares = append(o.middlewares, middlewares...)
	}
}
