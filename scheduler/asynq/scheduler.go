package asynq

import (
	"context"
	"errors"
	"fmt"
	"runtime"
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

type Config struct {
	Concurrency     int            `json:"concurrency" yaml:"concurrency"`
	Queues          map[string]int `json:"queues" yaml:"queues"`
	StrictPriority  bool           `json:"strict_priority" yaml:"strict_priority"`
	ShutdownTimeout time.Duration  `json:"shutdown_timeout" yaml:"shutdown_timeout"`
	Timezone        string         `json:"timezone" yaml:"timezone"`
}

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

func (s *Scheduler) Identifier() string {
	return "scheduler/asynq"
}

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

func (s *Scheduler) Handle(kind string, handler scheduler.PayloadHandlerFunc) error {
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
		return s.dispatch(ctx, kind, task)
	})
}

// dispatch runs the handler currently bound to kind. A task that was still
// queued when its handler was unregistered is dropped rather than failed: the
// registration is intentionally gone, so retrying it would never succeed.
func (s *Scheduler) dispatch(ctx context.Context, kind string, task *sasynq.Task) error {
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
		log.Warn("scheduler/asynq: no handler registered, dropping task", log.String("kind", kind))
		return nil
	}
}

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

func (s *Scheduler) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !s.state.CompareAndSwap(stateInit, stateRunning) {
		if s.state.Load() == stateClosed {
			return scheduler.ErrClosed
		}
		return scheduler.ErrAfterStart
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	if err := s.server.Start(s.mux); err != nil {
		s.state.Store(stateClosed)
		cancel()
		return err
	}
	if err := s.cron.Start(); err != nil {
		s.server.Shutdown()
		s.state.Store(stateClosed)
		cancel()
		return err
	}

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
			_ = s.closeClient()
			close(done)
		}()
		s.stopDone = done
	case stateStopping:
		// Shutdown already in progress; fall through to wait on the same channel.
	}
	done := s.stopDone
	s.mu.Unlock()

	if done == nil {
		return nil
	}
	select {
	case <-done:
		s.state.Store(stateClosed)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scheduler) Close() error {
	switch s.state.Load() {
	case stateClosed:
		return scheduler.ErrClosed
	case stateInit:
		if !s.state.CompareAndSwap(stateInit, stateClosed) {
			return nil
		}
		return s.closeClient()
	default:
		return s.Stop(context.Background())
	}
}

func (s *Scheduler) closeClient() error {
	return nil
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
