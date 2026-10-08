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
// location. The zero value uses standard five-field specs in the process
// local zone.
type Config struct {
	// Seconds requires a leading seconds field in every spec
	// ("0 */5 * * * *" instead of "*/5 * * * *").
	Seconds bool `json:"seconds" yaml:"seconds"`
	// Timezone is an IANA location name such as "UTC" or "Asia/Shanghai".
	// Empty uses time.Local.
	Timezone string `json:"timezone" yaml:"timezone"`
}

// Scheduler is a scheduler.Cron on robfig/cron/v3. It is not a
// scheduler.Scheduler: there is no Enqueue or Handle. It implements task.Task.
// Create it with NewScheduler; the zero value is not usable. Methods are safe
// for concurrent use.
type Scheduler struct {
	cron    *rcron.Cron
	entries map[string]rcron.EntryID

	mu            sync.Mutex
	state         atomic.Int32
	handlerCtx    context.Context
	cancel        context.CancelFunc
	handlerCancel context.CancelFunc
	stopDone      <-chan struct{}
	// stopWaiters counts Stop calls currently waiting on stopDone; the last one
	// to give up cancels handlerCtx.
	stopWaiters int
	// stopPending records a Stop that arrived before Start; Start then returns
	// nil without starting the cron.
	stopPending bool
}

// NewScheduler builds a Cron scheduler. Jobs run with SkipIfStillRunning and
// Recover. conf.Seconds enables a seconds field in cron specs. It returns an
// error only when conf.Timezone cannot be loaded. Nil options are ignored.
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
// scheduler.ErrAfterStart and after Close returns scheduler.ErrClosed. A nil
// handler or an invalid spec is rejected with an error. The handler receives a
// context that stays live until Stop has drained running jobs or the last
// waiting Stop has timed out (see Stop); a returned error or panic is logged
// and does not unschedule the job.
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

// Unregister removes the job named name. An unknown name is a no-op. It
// returns scheduler.ErrClosed after Close or a completed Stop.
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

// Start starts the cron and blocks until ctx is cancelled or Stop has
// drained running jobs, returning context.Canceled in the latter case and
// ctx.Err() in the former. A second Start returns scheduler.ErrAfterStart and
// Start after Close returns scheduler.ErrClosed. Start after a Stop that came
// first returns nil without starting the cron. Cancelling ctx without Stop
// leaves the cron running; the owner must call Stop or Close.
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
		stopPending := s.stopPending
		s.mu.Unlock()
		if stopPending {
			return nil
		}
		if state == stateClosed {
			return scheduler.ErrClosed
		}
		return scheduler.ErrAfterStart
	}

	// Handlers get a context of their own instead of runCtx. task.Group cancels
	// the context it passes to Start before it invokes Stop, so deriving handler
	// contexts from it handed every in-flight job a dead context the moment
	// shutdown began and reduced the drain below to a formality. WithoutCancel
	// keeps the parent's values but not its cancellation; cancelRun releases the
	// handlers once the drain has finished.
	runCtx, cancel := context.WithCancel(ctx)
	handlerCtx, handlerCancel := context.WithCancel(context.WithoutCancel(ctx))
	s.handlerCtx = handlerCtx
	s.cancel = cancel
	s.handlerCancel = handlerCancel
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

// Stop stops scheduling new runs and waits for in-flight jobs to return or
// ctx to expire. Handler contexts stay live while any Stop is still waiting,
// so jobs get the whole shutdown budget to finish. If ctx expires first, Stop
// returns ctx.Err(); when no other Stop (or Close) is still waiting, it also
// cancels the context of every in-flight handler, so a job that watches
// ctx.Done() can abort cleanly before the process exits. The drain continues
// in the background either way and a later Stop waits on the same drain. Stop
// before Start returns nil and closes the scheduler, so a later or racing
// Start returns nil without starting the cron (as with task.Group). Stop after
// the scheduler is closed returns nil.
func (s *Scheduler) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// The state transition and the recording of the shutdown-completion channel
	// happen under the mutex so that concurrent (and repeated) Stop/Close calls
	// all observe and wait on the same underlying shutdown signal.
	s.mu.Lock()
	switch s.state.Load() {
	case stateInit:
		// Nothing is running yet. Record the request under the same lock Start
		// uses for its transition so that a later (or racing) Start returns
		// without bringing the runtime up, as task.Group does. Dropping it left
		// a runtime that nobody would stop.
		s.state.Store(stateClosed)
		s.stopPending = true
		s.mu.Unlock()
		return nil
	case stateClosed:
		s.mu.Unlock()
		return nil
	case stateRunning:
		// First Stop call: trigger the underlying cron shutdown exactly once and
		// record the channel that reports when in-flight jobs have drained. The
		// handler context is not cancelled yet: doing so would hand every
		// in-flight handler a dead context and turn the drain into a formality.
		s.state.Store(stateStopping)
		stopDone := s.cron.Stop().Done()
		s.stopDone = stopDone
		// A dedicated watcher owns the final transition: if every Stop caller
		// times out before the drain finishes, no one would otherwise cancel
		// the run context, and Start would stay blocked forever.
		go func() {
			<-stopDone
			s.state.Store(stateClosed)
			s.cancelRun()
		}()
	case stateStopping:
		// Shutdown already in progress; fall through to wait on the same channel.
	}
	// Every path reaching here either installed stopDone while transitioning from
	// running or observed that same channel in stateStopping while holding mu.
	done := s.stopDone
	s.stopWaiters++
	s.mu.Unlock()

	select {
	case <-done:
		// Handlers have drained; release Start.
		s.mu.Lock()
		s.stopWaiters--
		s.mu.Unlock()
		s.state.Store(stateClosed)
		s.cancelRun()
		return nil
	case <-ctx.Done():
		// The handler context is shared, so it is cancelled only when no other
		// Stop with a longer budget is still waiting for the drain. The run
		// context stays live: the watcher releases Start once handlers return.
		s.mu.Lock()
		s.stopWaiters--
		last := s.stopWaiters == 0
		handlerCancel := s.handlerCancel
		s.mu.Unlock()
		if last && handlerCancel != nil {
			handlerCancel()
		}
		return ctx.Err()
	}
}

// cancelRun cancels the context handed to running handlers and the run
// context, which unblocks Start. It is called once the drain finishes;
// handlers that have already returned observe nothing, but any goroutine they
// spawned from the handler context is released. It is safe to call repeatedly.
func (s *Scheduler) cancelRun() {
	s.mu.Lock()
	cancel := s.cancel
	handlerCancel := s.handlerCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if handlerCancel != nil {
		handlerCancel()
	}
}

// Close closes a scheduler that was never started, or calls Stop with an
// unbounded context when it is running. A second Close (or Close after a
// completed Stop) returns scheduler.ErrClosed.
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
	if s.handlerCtx != nil {
		return s.handlerCtx
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
