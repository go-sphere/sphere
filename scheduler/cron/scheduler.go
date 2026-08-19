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

type Config struct {
	Seconds  bool   `json:"seconds" yaml:"seconds"`
	Timezone string `json:"timezone" yaml:"timezone"`
}

type Scheduler struct {
	cron    *rcron.Cron
	entries map[string]rcron.EntryID

	mu       sync.Mutex
	state    atomic.Int32
	runCtx   context.Context
	cancel   context.CancelFunc
	stopDone <-chan struct{}
}

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

func (s *Scheduler) Identifier() string {
	return "scheduler/cron"
}

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
	done := s.stopDone
	s.mu.Unlock()

	if done == nil {
		s.cancelRun()
		return nil
	}
	select {
	case <-done:
		// Handlers have drained; release Start.
		s.cancelRun()
		s.state.Store(stateClosed)
		return nil
	case <-ctx.Done():
		// The caller gave up waiting, so signal the stragglers to abort. A later
		// Stop call waits on the same channel and completes the transition.
		s.cancelRun()
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
	switch s.state.Load() {
	case stateClosed:
		return scheduler.ErrClosed
	case stateInit:
		s.state.Store(stateClosed)
		return nil
	default:
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
