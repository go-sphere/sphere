package telegram

import (
	"context"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type options struct {
	noRouteHandler bot.HandlerFunc
	errorHandler   ErrorHandlerFunc
	authExtractor  AuthExtractorFunc

	botOptions  []bot.Option
	middlewares []MiddlewareFunc
}

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
			log.Warnw("bot error", logfields.Error(err))
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

func WithErrorHandler(fn ErrorHandlerFunc) Option {
	return func(o *options) {
		o.errorHandler = fn
	}
}

func WithDefaultHandler(fn bot.HandlerFunc) Option {
	return func(o *options) {
		o.noRouteHandler = fn
	}
}

func WithAuthExtractor(extractor AuthExtractorFunc) Option {
	return func(o *options) {
		o.authExtractor = extractor
	}
}

func AppendBotOptions(opt ...bot.Option) Option {
	return func(o *options) {
		o.botOptions = append(o.botOptions, opt...)
	}
}

func AppendMiddlewares(middlewares ...MiddlewareFunc) Option {
	return func(o *options) {
		o.middlewares = append(o.middlewares, middlewares...)
	}
}
