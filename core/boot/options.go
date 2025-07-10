package boot

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

type Options struct {
	shutdownTimeout time.Duration
	beforeStart     []func() error
	beforeStop      []func() error
	afterStop       []func() error
	signals         []os.Signal
}

func newOptions(opts ...Option) *Options {
	opt := &Options{
		shutdownTimeout: 30 * time.Second,
		signals:         []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

type Option func(*Options)

func WithShutdownTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.shutdownTimeout = d
	}
}

func WithShutdownSignals(sigs ...os.Signal) Option {
	return func(o *Options) {
		o.signals = sigs
	}
}

func AddBeforeStart(f func() error) Option {
	return func(o *Options) {
		o.beforeStart = append(o.beforeStart, f)
	}
}

func AddBeforeStop(f func() error) Option {
	return func(o *Options) {
		o.beforeStop = append(o.beforeStop, f)
	}
}

func AddAfterStop(f func() error) Option {
	return func(o *Options) {
		o.afterStop = append(o.afterStop, f)
	}
}

func WithLoggerInit(ver string, conf *log.Options) Option {
	return func(o *Options) {
		o.beforeStart = append(o.beforeStart, func() error {
			log.Init(conf, logfields.String("version", ver))
			return nil
		})
		o.afterStop = append(o.afterStop, func() error {
			_ = log.Sync()
			return nil
		})
	}
}

func runHooks(hooks []func() error, hookType string) error {
	var errs []error
	for i, f := range hooks {
		if err := f(); err != nil {
			log.Errorf("Hook %s[%d] failed: %v", hookType, i, err)
			errs = append(errs, fmt.Errorf("%s hook[%d]: %w", hookType, i, err))
		}
	}
	return errors.Join(errs...)
}
