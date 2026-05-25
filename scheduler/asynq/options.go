package asynq

import (
	"context"

	sasynq "github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type Option func(*options)

type options struct {
	errorHandler sasynq.ErrorHandler
	logger       sasynq.Logger
	logLevel     sasynq.LogLevel
	client       *redis.Client
	serverConfig []func(*sasynq.Config)
}

// WithClient sets the Redis client used by asynq. When provided, the scheduler
// reuses the caller-owned Redis connection and will not close it.
func WithClient(client *redis.Client) Option {
	return func(o *options) {
		o.client = client
	}
}

func WithErrorHandler(h func(ctx context.Context, task *sasynq.Task, err error)) Option {
	return func(o *options) {
		if h != nil {
			o.errorHandler = sasynq.ErrorHandlerFunc(h)
		}
	}
}

func WithLogger(logger sasynq.Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}

func WithLogLevel(level sasynq.LogLevel) Option {
	return func(o *options) {
		o.logLevel = level
	}
}

func WithServerConfig(fn func(*sasynq.Config)) Option {
	return func(o *options) {
		if fn != nil {
			o.serverConfig = append(o.serverConfig, fn)
		}
	}
}
