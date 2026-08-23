package asynq

import (
	"context"

	sasynq "github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Option configures a Scheduler. WithClient is required.
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

// WithErrorHandler sets asynq's ErrorHandler for failed tasks. A nil handler
// is ignored.
func WithErrorHandler(h func(ctx context.Context, task *sasynq.Task, err error)) Option {
	return func(o *options) {
		if h != nil {
			o.errorHandler = sasynq.ErrorHandlerFunc(h)
		}
	}
}

// WithLogger replaces the default sphere log adapter. A nil logger is ignored.
func WithLogger(logger sasynq.Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// WithLogLevel sets asynq's log level. The default is InfoLevel.
func WithLogLevel(level sasynq.LogLevel) Option {
	return func(o *options) {
		o.logLevel = level
	}
}

// WithServerConfig appends a mutator for asynq.Config after defaults are
// applied. Multiple mutators run in registration order. A nil fn is ignored.
func WithServerConfig(fn func(*sasynq.Config)) Option {
	return func(o *options) {
		if fn != nil {
			o.serverConfig = append(o.serverConfig, fn)
		}
	}
}
