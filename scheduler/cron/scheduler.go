// Package cron is a scheduler.Cron on robfig/cron/v3. It is not a full
// scheduler.Scheduler: there is no Enqueue/Handle.
//
// Implements task.Task. Start blocks until cancel; Stop drains jobs without
// cancelling in-flight handler ctx until drain completes. Cancelling Start's
// ctx without Stop leaves the runtime live. Optional seconds field and
// timezone. Chain: SkipIfStillRunning + Recover. Duplicate Register returns
// scheduler.ErrDuplicateName; Register after Start returns ErrAfterStart;
// unknown Unregister is a no-op.
package cron

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/scheduler"
	rcron "github.com/robfig/cron/v3"
)

const (
	stateInit int32 = iota
	stateRunning
	stateStopping
	stateClosed
)

// Config selects an optional seconds field in cron specs and a timezone
// location. Empty Timezone uses the process local zone.
type Config struct {
	Seconds  bool   `json:"seconds" yaml:"seconds"`
	Timezone string `json:"timezone" yaml:"timezone"`
}

// Scheduler is a scheduler.Cron on robfig/cron/v3. It is not a
// scheduler.Scheduler: there is no Enqueue or Handle. It implements task.Task.
type Scheduler struct {
	cron    *rcron.Cron
	entries map[string]rcron.EntryID

	mu       sync.Mutex
	state    atomic.Int32
	runCtx   context.Context
	cancel   context.CancelFunc
	stopDone <-chan struct{}
}

// NewScheduler builds a Cron scheduler. Jobs run with SkipIfStillRunning and
// Recover. conf.Seconds enables a seconds field in cron specs.
func NewScheduler(conf Config, opts ...Option) (*Scheduler, error) {
	applied := options{logger: cronLogger{}}
	for _, opt := range opts {
		if opt != nil {
			opt(&applied)
		}
	}

	cronOpts := []rcron.Option{
		rcron.WithChain(
			rcron.SkipIfStillRunning(applied.logger),
			rcron.Recover(applied.logger),
		),
	}
	if conf.Seconds {
		cronOpts = append(cronOpts, rcron.WithSeconds())
	}
	if conf.Timezone != "" {
		loc, err := time.LoadLocation(conf.Timezone)
		if err != nil {
			return nil, fmt.Errorf("scheduler/cron: load timezone: %w", err)
		}
		cronOpts = append(cronOpts, rcron.WithLocation(loc))
	}

	return &Scheduler{
		cron:    rcron.New(cronOpts...),
		entries: make(map[string]rcron.EntryID),
	}, nil
}

// Identifier returns "scheduler/cron".
func (s *Scheduler) Identifier() string {
	return "scheduler/cron"
}

// Register adds a periodic job named name with cron spec. Duplicate names
// return scheduler.ErrDuplicateName. Register after Start returns
// ErrAfterStart. A nil handler is rejected.
func (s *Scheduler) Register(name, spec string, handler scheduler.HandlerFunc) error {
	if handler == nil {
		return fmt.Errorf("scheduler/cron: nil handler")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.state.Load() {
	case stateClosed:
		return scheduler.ErrClosed
	case stateInit:
	default:
		return scheduler.ErrAfterStart
	}
	if _, ok := s.entries[name]; ok {
		return scheduler.ErrDuplicateName
	}

	wrapped := scheduler.RecoverHandler(handler)
	id, err := s.cron.AddFunc(spec, func() {
		ctx := s.lifecycleContext()
		if err := wrapped(ctx); err != nil {
			log.ErrorContext(ctx, "cron handler failed", log.String("name", name), log.Err(err))
		}
	})
	if err != nil {
		return err
	}
	s.entries[name] = id
	return nil
}

// Unregister removes the job named name. An unknown name is a no-op.
func (s *Scheduler) Unregister(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Load() == stateClosed {
		return scheduler.ErrClosed
	}
	id, ok := s.entries[name]
	if !ok {
		return nil
	}
	s.cron.Remove(id)
	delete(s.entries, name)
	return nil
}

func (s *Scheduler) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// The state transition and the underlying startup must happen together under
	// the mutex. If a Stop interleaved between them it would observe stateRunning
	// and stop a cron that had not been started, then Start would bring it up with
	// the scheduler already marked closed and nobody left to stop it.
	s.mu.Lock()
	if !s.state.CompareAndSwap(stateInit, stateRunning) {
		state := s.state.Load()
		s.mu.Unlock()
		if state == stateClosed {
			return scheduler.ErrClosed
		}
		return scheduler.ErrAfterStart
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.runCtx = runCtx
	s.cancel = cancel
	s.cron.Start()
	s.mu.Unlock()

	<-runCtx.Done()
	// Stop is the cleanup half of the Task contract. Returning here lets the
	// runner (Group, boot.Run) apply its shutdown budget to Stop. A Start that
	// called Stop with a detached context made that budget unreachable: the
	// runner waited on Start, which waited on an unbounded drain. Cancelling
	// the parent context without Stop leaves the runtime live — call Stop.
	return runCtx.Err()
}

func (s *Scheduler) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// The state transition and the recording of the shutdown-completion channel
	// happen under the mutex so that concurrent (and repeated) Stop/Close calls
	// all observe and wait on the same underlying shutdown signal.
	s.mu.Lock()
	switch s.state.Load() {
	case stateInit, stateClosed:
		s.mu.Unlock()
		return nil
	case stateRunning:
		// First Stop call: trigger the underlying cron shutdown exactly once and
		// record the channel that reports when in-flight jobs have drained. The
		// run context is deliberately NOT cancelled here — cancelling first would
		// hand every in-flight handler a dead context and turn the drain below
		// into a formality. It is cancelled once they finish, or when the caller
		// stops waiting.
		s.state.Store(stateStopping)
		s.stopDone = s.cron.Stop().Done()
	case stateStopping:
		// Shutdown already in progress; fall through to wait on the same channel.
	}
	// Every path reaching here either installed stopDone while transitioning from
	// running or observed that same channel in stateStopping while holding mu.
	done := s.stopDone
	s.mu.Unlock()

	select {
	case <-done:
		// Handlers have drained; release Start.
		s.cancelRun()
		s.state.Store(stateClosed)
		return nil
	case <-ctx.Done():
		// This caller gave up waiting. The run context is deliberately left
		// alone: it is shared by every in-flight handler, so cancelling it here
		// would abort work that another Stop — one with a longer budget, or the
		// staged shutdown the group is running — is still legitimately waiting
		// to drain. One caller's deadline is not everyone's. The drain continues
		// in the background and whichever Stop observes it finish releases Start.
		return ctx.Err()
	}
}

// cancelRun cancels the context handed to running handlers, which also unblocks
// Start. It is safe to call repeatedly.
func (s *Scheduler) cancelRun() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Scheduler) Close() error {
	// The state is read and written under the same mutex Start uses for its own
	// transition. Reading it unlocked let a Close that observed stateInit race a
	// concurrent Start and then unconditionally overwrite stateRunning with
	// stateClosed. Every later Stop short-circuited on that state and returned
	// nil immediately, so the cron kept ticking with nothing able to stop it and
	// Start stayed blocked forever.
	s.mu.Lock()
	switch s.state.Load() {
	case stateClosed:
		s.mu.Unlock()
		return scheduler.ErrClosed
	case stateInit:
		s.state.Store(stateClosed)
		s.mu.Unlock()
		return nil
	default:
		s.mu.Unlock()
		return s.Stop(context.Background())
	}
}

func (s *Scheduler) lifecycleContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runCtx != nil {
		return s.runCtx
	}
	return context.Background()
}

type cronLogger struct{}

func (cronLogger) Info(msg string, keysAndValues ...any) {
	log.Info(msg, log.Any("cron", keysAndValues))
}

func (cronLogger) Error(err error, msg string, keysAndValues ...any) {
	log.Error(msg, log.Err(err), log.Any("cron", keysAndValues))
}
