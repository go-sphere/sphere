package scheduler

import (
	"context"
	"errors"
	"io"
)

// HandlerFunc is shared by cron-style periodic tasks and asynchronous tasks
// that do not need a payload.
type HandlerFunc func(ctx context.Context) error

// PayloadHandlerFunc handles asynchronous tasks routed by kind.
type PayloadHandlerFunc func(ctx context.Context, payload []byte) error

// Cron registers periodic tasks using cron expressions. Implementations must
// reject duplicate names with ErrDuplicateName and treat unknown unregisters as
// noops.
type Cron interface {
	Register(name, spec string, handler HandlerFunc) error
	Unregister(name string) error
}

// Producer enqueues asynchronous tasks.
type Producer interface {
	Enqueue(ctx context.Context, kind string, payload []byte, opts ...EnqueueOption) (taskID string, err error)
}

// Consumer registers asynchronous task handlers. Implementations must reject
// registrations after Start with ErrAfterStart.
type Consumer interface {
	Handle(kind string, handler PayloadHandlerFunc) error
}

// Scheduler is the complete scheduler capability set.
type Scheduler interface {
	Cron
	Producer
	Consumer
	io.Closer
}

var (
	ErrDuplicateName = errors.New("scheduler: duplicate task name")
	ErrAfterStart    = errors.New("scheduler: cannot register after start")
	ErrClosed        = errors.New("scheduler: closed")
)
