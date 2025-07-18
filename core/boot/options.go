package boot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

type Hook = func(context.Context) error

type options struct {
	shutdownTimeout time.Duration
	beforeStart     []Hook
	beforeStop      []Hook
	afterStop       []Hook
	signals         []os.Signal
}

func newOptions(opts ...Option) *options {
	opt := &options{
		shutdownTimeout: 30 * time.Second,
		signals:         []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
	}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

type Option func(*options)

func WithShutdownTimeout(d time.Duration) Option {
	return func(o *options) {
		o.shutdownTimeout = d
	}
}

func WithShutdownSignals(sigs ...os.Signal) Option {
	return func(o *options) {
		o.signals = sigs
	}
}

func AddBeforeStart(f Hook) Option {
	return func(o *options) {
		o.beforeStart = append(o.beforeStart, f)
	}
}

func AddBeforeStop(f Hook) Option {
	return func(o *options) {
		o.beforeStop = append(o.beforeStop, f)
	}
}

func AddAfterStop(f Hook) Option {
	return func(o *options) {
		o.afterStop = append(o.afterStop, f)
	}
}

func WithLoggerInit(ver string, conf *log.Options) Option {
	return func(o *options) {
		o.beforeStart = append(o.beforeStart, func(context.Context) error {
			log.Init(conf, logfields.String("version", ver))
			return nil
		})
		o.afterStop = append(o.afterStop, func(context.Context) error {
			_ = log.Sync()
			return nil
		})
	}
}

func runHooks(ctx context.Context, hooks []Hook, hookType string) error {
	var errs []error
	for i, f := range hooks {
		if err := f(ctx); err != nil {
			log.Errorf("Hook %s[%d] failed: %v", hookType, i, err)
			errs = append(errs, fmt.Errorf("%s hook[%d]: %w", hookType, i, err))
		}
	}
	return errors.Join(errs...)
}
