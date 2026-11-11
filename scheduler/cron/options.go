package cron

import rcron "github.com/robfig/cron/v3"

// Option configures a Scheduler.
type Option func(*options)

type options struct {
	logger rcron.Logger
}

// WithLogger replaces the default sphere log adapter used by SkipIfStillRunning
// and Recover. A nil logger is ignored.
func WithLogger(logger rcron.Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}
