package cron

import rcron "github.com/robfig/cron/v3"

type Option func(*options)

type options struct {
	logger rcron.Logger
}

func WithLogger(logger rcron.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}
