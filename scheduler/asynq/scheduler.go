// Package asynq is a scheduler.Scheduler on hibiken/asynq over an injected
// *redis.Client (WithClient is required; Redis is never closed here).
//
// Periodic tasks require the "default" queue. Register stores cron jobs
// under mux kind scheduler.cron:<name>. Handle routes by exact task.Type(),
// not mux prefix: user.email does not receive user.email.welcome.
// Unregistered kinds error so asynq retries/archives. Mux entries are not
// unmounted. Implements task.Task. Stop returning ctx.Err() does not mean
// asynq is idle — wait for Stop/Close to return nil before closing Redis.
package asynq

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/scheduler"
	sasynq "github.com/hibiken/asynq"
)

const (
	defaultQueue = "default"
)

const (
	stateInit int32 = iota
	stateRunning
	stateStopping
	stateClosed
)

// Config configures the asynq worker and periodic scheduler.
//
// Queues must include "default"; periodic tasks are enqueued there. An empty
// map becomes {"default": 1}. Concurrency defaults to runtime.NumCPU.
// ShutdownTimeout defaults to 30s. Timezone is the cron location; empty uses
// time.Local.
type Config struct {
	Concurrency     int            `json:"concurrency" yaml:"concurrency"`
	Queues          map[string]int `json:"queues" yaml:"queues"`
	StrictPriority  bool           `json:"strict_priority" yaml:"strict_priority"`
	ShutdownTimeout time.Duration  `json:"shutdown_timeout" yaml:"shutdown_timeout"`
	Timezone        string         `json:"timezone" yaml:"timezone"`
}

// Scheduler is a scheduler.Scheduler backed by hibiken/asynq. It also
// implements task.Task. WithClient is required; the Redis client is never
// closed here.
type Scheduler struct {
	conf    Config
	client  *sasynq.Client
	server  *sasynq.Server
	mux     *sasynq.ServeMux
	cron    *sasynq.Scheduler
	options options

	mu       sync.Mutex
	handlers map[string]scheduler.PayloadHandlerFunc
	// cronFuncs holds the periodic handlers keyed by their generated task kind,
	// mirroring cronIDs (which is keyed by the user-facing name).
	cronFuncs map[string]scheduler.HandlerFunc
	// mounted records the kinds already attached to the ServeMux. asynq's mux
	// panics on a duplicate pattern and cannot detach a handler, so each kind is
	// mounted at most once for the lifetime of the scheduler.
	mounted  map[string]struct{}
	cronIDs  map[string]string
	state    atomic.Int32
	cancel   context.CancelFunc
	stopDone <-chan struct{}
}

// NewScheduler builds a Scheduler. WithClient is required. Queues must
// include "default". The injected Redis client is not closed by Close.
func NewScheduler(conf Config, opts ...Option) (*Scheduler, error) {
	conf = applyDefaults(conf)
	if _, ok := conf.Queues[defaultQueue]; !ok {
		return nil, fmt.Errorf("scheduler/asynq: queues must include %q for periodic tasks", defaultQueue)
	}

	applied := options{
		logger:   sphereLogAdapter{},
		logLevel: sasynq.InfoLevel,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&applied)
		}
	}

	client, server, cron, err := newAsynqRuntime(conf, applied)
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		conf:      conf,
		client:    client,
		server:    server,
		mux:       sasynq.NewServeMux(),
		cron:      cron,
		options:   applied,
		handlers:  make(map[string]scheduler.PayloadHandlerFunc),
		cronFuncs: make(map[string]scheduler.HandlerFunc),
		mounted:   make(map[string]struct{}),
		cronIDs:   make(map[string]string),
	}, nil
}

func newAsynqRuntime(cfg Config, applied options) (*sasynq.Client, *sasynq.Server, *sasynq.Scheduler, error) {
	if applied.client == nil {
		return nil, nil, nil, errors.New("redis client is required")
	}

	serverCfg := sasynq.Config{
		Concurrency:     cfg.Concurrency,
		Queues:          cfg.Queues,
		StrictPriority:  cfg.StrictPriority,
		ShutdownTimeout: cfg.ShutdownTimeout,
		Logger:          applied.logger,
		LogLevel:        applied.logLevel,
		ErrorHandler:    applied.errorHandler,
	}
	for _, fn := range applied.serverConfig {
		fn(&serverCfg)
	}

	location := time.Local
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("scheduler/asynq: load timezone: %w", err)
		}
		location = loc
	}

	schedulerOpts := &sasynq.SchedulerOpts{
		Location: location,
		Logger:   applied.logger,
		LogLevel: applied.logLevel,
	}

	return sasynq.NewClientFromRedisClient(applied.client),
		sasynq.NewServerFromRedisClient(applied.client, serverCfg),
		sasynq.NewSchedulerFromRedisClient(applied.client, schedulerOpts),
		nil
}

// Identifier returns "scheduler/asynq".
func (s *Scheduler) Identifier() string {
	return "scheduler/asynq"
}

// Register schedules a periodic job named name with cron spec. The job is
// stored under mux kind scheduler.cron:<name>. Duplicate names, collisions
// with Handle kinds, and Register after Start are errors. A nil handler is
// rejected.
func (s *Scheduler) Register(name, spec string, handler scheduler.HandlerFunc) error {
	if handler == nil {
		return fmt.Errorf("scheduler/asynq: nil handler")
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
	if _, ok := s.cronIDs[name]; ok {
		return scheduler.ErrDuplicateName
	}

	kind := cronKind(name)
	// A cron name and an async kind share one mux pattern, so a collision between
	// the two must be rejected rather than silently shadowing one of them.
	if _, ok := s.handlers[kind]; ok {
		return scheduler.ErrDuplicateName
	}

	entryID, err := s.cron.Register(spec, sasynq.NewTask(kind, nil), sasynq.MaxRetry(0))
	if err != nil {
		return err
	}
	s.cronFuncs[kind] = scheduler.RecoverHandler(handler)
	s.cronIDs[name] = entryID
	s.mount(kind)
	return nil
}

// Unregister drops the periodic job named name. An unknown name is a no-op.
// The ServeMux entry is left mounted; the kind is inert until registered again.
func (s *Scheduler) Unregister(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Load() == stateClosed {
		return scheduler.ErrClosed
	}
	entryID, ok := s.cronIDs[name]
	if !ok {
		return nil
	}
	if err := s.cron.Unregister(entryID); err != nil {
		return err
	}
	delete(s.cronIDs, name)
	// The mux entry for this kind intentionally stays mounted (asynq cannot
	// detach it); dropping the handler is what makes the kind inert, and it
	// allows the same name to be registered again later.
	delete(s.cronFuncs, cronKind(name))
	return nil
}

// Handle binds handler to an exact task kind. asynq prefix matching is not
// used: user.email does not receive user.email.welcome. Empty kind, duplicate
// kind, and Handle after Start are errors. Unregistered kinds error at
// dispatch so asynq retries or archives.
func (s *Scheduler) Handle(kind string, handler scheduler.PayloadHandlerFunc) error {
	if handler == nil {
		return fmt.Errorf("scheduler/asynq: nil handler")
	}
	// asynq's ServeMux panics on an empty or blank pattern. A kind read from
	// configuration can easily be empty, and this runs inside the application
	// builder, so the panic takes the process down at startup instead of
	// reporting a configuration error the way the nil-handler check does.
	if strings.TrimSpace(kind) == "" {
		return fmt.Errorf("scheduler/asynq: empty task kind")
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
	if _, ok := s.handlers[kind]; ok {
		return scheduler.ErrDuplicateName
	}
	if _, ok := s.cronFuncs[kind]; ok {
		return scheduler.ErrDuplicateName
	}

	s.handlers[kind] = scheduler.RecoverPayloadHandler(handler)
	s.mount(kind)
	return nil
}

// mount attaches a dispatcher for kind to the ServeMux exactly once. asynq's
// ServeMux panics on a duplicate pattern and provides no way to detach a
// handler, so the mux entry must outlive individual Register/Unregister calls
// and the dispatcher resolves the currently bound handler at delivery time.
// Callers must hold s.mu.
func (s *Scheduler) mount(kind string) {
	if _, ok := s.mounted[kind]; ok {
		return
	}
	s.mounted[kind] = struct{}{}
	s.mux.HandleFunc(kind, func(ctx context.Context, task *sasynq.Task) error {
		return s.dispatch(ctx, task)
	})
}

// dispatch runs the handler bound to the task's own kind.
//
// It resolves the handler from task.Type() rather than from the pattern the mux
// matched, because asynq's ServeMux falls back to longest-prefix matching when
// no pattern matches exactly. Keying on the pattern therefore sent an
// unregistered kind to whichever registered kind happened to be a prefix of it
// — "user.email.welcome.v2" landing in the handler for "user.email", with that
// handler's payload decoding applied to a task it was never meant to see. The
// Consumer contract routes by kind and defines no hierarchy, and hierarchical
// names are exactly what the documentation's own examples use.
func (s *Scheduler) dispatch(ctx context.Context, task *sasynq.Task) error {
	kind := task.Type()

	s.mu.Lock()
	cronHandler := s.cronFuncs[kind]
	payloadHandler := s.handlers[kind]
	s.mu.Unlock()

	switch {
	case cronHandler != nil:
		return cronHandler(ctx)
	case payloadHandler != nil:
		return payloadHandler(ctx, task.Payload())
	default:
		// Nothing is bound to this kind: either it was unregistered while the
		// task sat in the queue, or the mux prefix-matched a kind that was never
		// registered at all. Report it so asynq applies its normal failure
		// handling (retry, then archive) instead of acknowledging a task no one
		// processed, which erased it with no trace beyond a log line.
		return fmt.Errorf("scheduler/asynq: no handler registered for kind %q", kind)
	}
}

// Enqueue publishes a task of kind with payload and returns the asynq task
// ID. Unique collisions are wrapped as scheduler.ErrDuplicateName.
func (s *Scheduler) Enqueue(ctx context.Context, kind string, payload []byte, opts ...scheduler.EnqueueOption) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.state.Load() == stateClosed {
		return "", scheduler.ErrClosed
	}

	applied := scheduler.ApplyEnqueueOptions(opts...)
	asynqOpts := make([]sasynq.Option, 0, 7)
	if applied.Delay > 0 {
		asynqOpts = append(asynqOpts, sasynq.ProcessIn(applied.Delay))
	}
	if !applied.Deadline.IsZero() {
		asynqOpts = append(asynqOpts, sasynq.Deadline(applied.Deadline))
	}
	if applied.MaxRetry > 0 {
		asynqOpts = append(asynqOpts, sasynq.MaxRetry(applied.MaxRetry))
	}
	if applied.Queue != "" {
		asynqOpts = append(asynqOpts, sasynq.Queue(applied.Queue))
	}
	if applied.UniqueFor > 0 {
		asynqOpts = append(asynqOpts, sasynq.Unique(applied.UniqueFor))
	}
	if applied.Retention > 0 {
		asynqOpts = append(asynqOpts, sasynq.Retention(applied.Retention))
	}
	if applied.TaskID != "" {
		asynqOpts = append(asynqOpts, sasynq.TaskID(applied.TaskID))
	}

	info, err := s.client.EnqueueContext(ctx, sasynq.NewTask(kind, payload), asynqOpts...)
	if errors.Is(err, sasynq.ErrDuplicateTask) {
		return "", fmt.Errorf("%w: %v", scheduler.ErrDuplicateName, err)
	}
	if err != nil {
		return "", err
	}
	if info == nil {
		return "", fmt.Errorf("scheduler/asynq: enqueue returned nil task info")
	}
	return info.ID, nil
}

// Start starts the asynq server and cron scheduler, then blocks until ctx is
// cancelled. The return is ctx.Err(); it does not mean asynq has drained.
// Cancelling ctx without Stop leaves the runtime live — call Stop or Close.
func (s *Scheduler) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// The state transition and the underlying startup must happen together under
	// the mutex. If a Stop interleaved between them it would observe stateRunning
	// and "shut down" a server asynq still considers new — both Stop and Shutdown
	// return immediately in that state — and then Start would bring the server up
	// with the scheduler already marked closed and nobody left to stop it. The
	// leaked server keeps a subscriber goroutine on the injected Redis client,
	// which panics inside asynq once that client is closed.
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
	s.cancel = cancel

	if err := s.server.Start(s.mux); err != nil {
		s.state.Store(stateClosed)
		s.mu.Unlock()
		cancel()
		return err
	}
	if err := s.cron.Start(); err != nil {
		s.state.Store(stateClosed)
		// Unlock before Shutdown: it waits for in-flight workers, and a worker
		// blocked in dispatch (which takes s.mu) would deadlock against us.
		s.mu.Unlock()
		s.server.Shutdown()
		cancel()
		return err
	}
	s.mu.Unlock()

	<-runCtx.Done()
	// Stop is the cleanup half of the Task contract. Returning here lets the
	// runner (Group, boot.Run) apply its shutdown budget to Stop. A Start that
	// called Stop with a detached context made that budget unreachable: the
	// runner waited on Start, which waited on an unbounded drain. Cancelling
	// the parent context without Stop leaves the runtime live — call Stop.
	return runCtx.Err()
}

// Stop shuts the scheduler down and waits for the asynq server to drain.
//
// A nil return means the drain finished. If ctx expires first Stop returns
// ctx.Err() and the shutdown continues in the background — the scheduler is not
// yet quiesced. Because the Redis client is injected and not owned (see
// WithClient), callers must not close that client until a Stop returns nil or
// Close returns: asynq's subscriber goroutine is still using it, and closing it
// underneath asynq panics inside the library. Retrying Stop with a longer budget
// waits on the same shutdown.
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
		// First Stop call: trigger the underlying shutdown exactly once and record
		// the channel that reports when the server has fully drained.
		s.state.Store(stateStopping)
		if s.cancel != nil {
			s.cancel()
		}
		s.cron.Shutdown()
		s.server.Stop()

		done := make(chan struct{})
		go func() {
			s.server.Shutdown()
			close(done)
		}()
		s.stopDone = done
	case stateStopping:
		// Shutdown already in progress; fall through to wait on the same channel.
	}
	// Every path reaching here either installed stopDone while transitioning from
	// running or observed that same channel in stateStopping while holding mu.
	done := s.stopDone
	s.mu.Unlock()

	select {
	case <-done:
		s.state.Store(stateClosed)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close shuts the scheduler down, waiting for an in-flight drain to finish. The
// Redis client is injected and not owned (see WithClient), so it is deliberately
// left open for its owner to close; asynq refuses to close a shared connection
// anyway.
func (s *Scheduler) Close() error {
	// The state is read and written under the same mutex Start uses for its own
	// transition. Reading it unlocked let a Close that observed stateInit race a
	// concurrent Start: the CAS then failed silently, Close still reported
	// success, and the caller — told by this type's own contract that it may
	// release the injected Redis client once Close returns — pulled the client
	// out from under a server asynq had just brought up.
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

func applyDefaults(cfg Config) Config {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = runtime.NumCPU()
	}
	if len(cfg.Queues) == 0 {
		cfg.Queues = map[string]int{defaultQueue: 1}
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	return cfg
}

func cronKind(name string) string {
	return "scheduler.cron:" + name
}

type sphereLogAdapter struct{}

func (sphereLogAdapter) Debug(args ...any) {
	log.Debug(fmt.Sprint(args...))
}

func (sphereLogAdapter) Info(args ...any) {
	log.Info(fmt.Sprint(args...))
}

func (sphereLogAdapter) Warn(args ...any) {
	log.Warn(fmt.Sprint(args...))
}

func (sphereLogAdapter) Error(args ...any) {
	log.Error(fmt.Sprint(args...))
}

func (sphereLogAdapter) Fatal(args ...any) {
	log.Error(fmt.Sprint(args...))
}
