// Package scheduler is the contract for periodic jobs and asynchronous task
// queues. Drivers expose only the capabilities they support.
//
//   - Cron: Register/Unregister periodic jobs by name and cron spec.
//   - Producer: Enqueue named payloads.
//   - Consumer: Handle payloads by exact kind.
//   - Scheduler: Cron + Producer + Consumer + io.Closer.
//
// Drivers: scheduler/cron wraps robfig/cron (Cron only). scheduler/asynq
// wraps hibiken/asynq (full Scheduler). Both also implement task.Task, so
// they belong in a boot.Run builder. Start blocks until Stop; cancelling
// Start's context without Stop leaves the runtime live.
//
// Neither driver coordinates periodic jobs across processes. N replicas run
// each periodic job N times per tick. Run the scheduler as a single replica,
// or make handlers idempotent. Enqueue/Handle have no such restriction: the
// queue delivers each task to one consumer.
//
// Handlers are routed by exact kind, not prefix. A kind with no handler is
// a failure so asynq can retry/archive. Register after Start returns
// ErrAfterStart. Duplicate names return ErrDuplicateName. Unknown
// Unregister is a no-op.
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
// reject duplicate names with ErrDuplicateName, reject Register after Start
// or Close with ErrAfterStart / ErrClosed, and treat unknown Unregister as a
// no-op.
type Cron interface {
	Register(name, spec string, handler HandlerFunc) error
	Unregister(name string) error
}

// Producer enqueues asynchronous tasks.
type Producer interface {
	Enqueue(ctx context.Context, kind string, payload []byte, opts ...EnqueueOption) (taskID string, err error)
}

// Consumer registers asynchronous task handlers by exact kind. Implementations
// must reject Handle after Start with ErrAfterStart and after Close with
// ErrClosed.
type Consumer interface {
	Handle(kind string, handler PayloadHandlerFunc) error
}

// Scheduler is Cron + Producer + Consumer + io.Closer. It is not task.Task;
// drivers add Identifier/Start/Stop separately. cron.Scheduler implements
// Cron only.
type Scheduler interface {
	Cron
	Producer
	Consumer
	io.Closer
}

var (
	// ErrDuplicateName is returned by Cron.Register when name is already registered.
	ErrDuplicateName = errors.New("scheduler: duplicate task name")
	// ErrAfterStart is returned by Register/Handle after Start has been called.
	ErrAfterStart = errors.New("scheduler: cannot register after start")
	// ErrClosed is returned by Register/Enqueue/Handle after Close.
	ErrClosed = errors.New("scheduler: closed")
)
