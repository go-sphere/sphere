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
	Concurrency     int            `json:"concurrency"`
	Queues          map[string]int `json:"queues"`
	StrictPriority  bool           `json:"strict_priority"`
	ShutdownTimeout time.Duration  `json:"shutdown_timeout"`
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
	cronIDs  map[string]string
	state    atomic.Int32
	cancel   context.CancelFunc
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
		conf:     conf,
		client:   client,
		server:   server,
		mux:      sasynq.NewServeMux(),
		cron:     cron,
		options:  applied,
		handlers: make(map[string]scheduler.PayloadHandlerFunc),
		cronIDs:  make(map[string]string),
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

	schedulerOpts := &sasynq.SchedulerOpts{
		Location: time.Local,
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
	entryID, err := s.cron.Register(spec, sasynq.NewTask(kind, nil), sasynq.MaxRetry(0))
	if err != nil {
		return err
	}
	wrapped := scheduler.RecoverHandler(handler)
	s.mux.HandleFunc(kind, func(ctx context.Context, task *sasynq.Task) error {
		_ = task
		return wrapped(ctx)
	})
	s.cronIDs[name] = entryID
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

	wrapped := scheduler.RecoverPayloadHandler(handler)
	s.handlers[kind] = wrapped
	s.mux.HandleFunc(kind, func(ctx context.Context, task *sasynq.Task) error {
		return wrapped(ctx, task.Payload())
	})
	return nil
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
	if !s.state.CompareAndSwap(stateRunning, stateStopping) {
		return nil
	}

	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	s.cron.Shutdown()
	s.server.Stop()

	done := make(chan struct{})
	go func() {
		s.server.Shutdown()
		_ = s.closeClient()
		close(done)
	}()

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
