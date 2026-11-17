package cron

import rcron "github.com/robfig/cron/v3"

// Option configures a Scheduler.
type Option func(*options)

type options struct {
	logger rcron.Logger
}

// WithLogger replaces the default sphere log adapter used by the cron
// SkipIfStillRunning and Recover chain wrappers (handler errors still log via
// sphere's package log, and robfig/cron's internal schedule loop keeps its
// default logger). A nil logger is ignored.
func WithLogger(logger rcron.Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}
