package testtask

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"
)

// AutoClose simulates a task that starts and then fails with an error after a delay.
type AutoClose struct {
	delay time.Duration
}

func NewAutoClose() *AutoClose {
	return &AutoClose{delay: 3 * time.Second}
}

func NewAutoCloseWithDelay(delay time.Duration) *AutoClose {
	return &AutoClose{delay: delay}
}

func (a *AutoClose) Identifier() string {
	return "autoclose"
}

func (a *AutoClose) Start(ctx context.Context) error {
	time.Sleep(a.delay)
	return errors.New("simulated error for autoclose task")
}

func (a *AutoClose) Stop(ctx context.Context) error {
	return nil
}

// AutoPanic simulates a task that panics during startup.
type AutoPanic struct {
	delay time.Duration
}

func NewAutoPanic() *AutoPanic {
	return &AutoPanic{delay: 3 * time.Second}
}

func NewAutoPanicWithDelay(delay time.Duration) *AutoPanic {
	return &AutoPanic{delay: delay}
}

func (a *AutoPanic) Identifier() string {
	return "autopanic"
}

func (a *AutoPanic) Start(ctx context.Context) error {
	time.Sleep(a.delay)
	panic("simulated panic for autopanic task")
}

func (a *AutoPanic) Stop(ctx context.Context) error {
	return nil
}

// ServerExample simulates a long-running HTTP server task.
type ServerExample struct {
	server *http.Server
}

func NewServerExample() *ServerExample {
	return &ServerExample{
		server: &http.Server{
			Addr: ":0",
		},
	}
}

func (s *ServerExample) Identifier() string {
	return "serverexample"
}

func (s *ServerExample) Start(ctx context.Context) error {
	err := s.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
		return errors.Join(ctx.Err(), err)
	}
	return err
}

func (s *ServerExample) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// SuccessTask simulates a task that starts successfully and runs until stopped.
type SuccessTask struct {
	id      string
	started atomic.Bool
	stopped atomic.Bool
}

func NewSuccessTask(id string) *SuccessTask {
	return &SuccessTask{id: id}
}

func (s *SuccessTask) Identifier() string {
	return s.id
}

func (s *SuccessTask) Start(ctx context.Context) error {
	s.started.Store(true)
	<-ctx.Done()
	return ctx.Err()
}

func (s *SuccessTask) Stop(ctx context.Context) error {
	s.stopped.Store(true)
	return nil
}

func (s *SuccessTask) IsStarted() bool {
	return s.started.Load()
}

func (s *SuccessTask) IsStopped() bool {
	return s.stopped.Load()
}

// SlowStartTask simulates a task with slow startup.
type SlowStartTask struct {
	id        string
	startTime time.Duration
	started   atomic.Bool
	stopped   atomic.Bool
}

func NewSlowStartTask(id string, startTime time.Duration) *SlowStartTask {
	return &SlowStartTask{
		id:        id,
		startTime: startTime,
	}
}

func (s *SlowStartTask) Identifier() string {
	return s.id
}

func (s *SlowStartTask) Start(ctx context.Context) error {
	select {
	case <-time.After(s.startTime):
		s.started.Store(true)
		<-ctx.Done()
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *SlowStartTask) Stop(ctx context.Context) error {
	s.stopped.Store(true)
	return nil
}

func (s *SlowStartTask) IsStarted() bool {
	return s.started.Load()
}

func (s *SlowStartTask) IsStopped() bool {
	return s.stopped.Load()
}

// FailingStartTask simulates a task that fails to start immediately.
type FailingStartTask struct {
	id      string
	err     error
	stopped atomic.Bool
}

func NewFailingStartTask(id string, err error) *FailingStartTask {
	return &FailingStartTask{
		id:  id,
		err: err,
	}
}

func (f *FailingStartTask) Identifier() string {
	return f.id
}

func (f *FailingStartTask) Start(ctx context.Context) error {
	return f.err
}

func (f *FailingStartTask) Stop(ctx context.Context) error {
	f.stopped.Store(true)
	return nil
}

func (f *FailingStartTask) IsStopped() bool {
	return f.stopped.Load()
}

// FailingStopTask simulates a task that fails to stop gracefully.
type FailingStopTask struct {
	id        string
	stopError error
	started   atomic.Bool
	stopped   atomic.Bool
}

func NewFailingStopTask(id string, stopError error) *FailingStopTask {
	return &FailingStopTask{
		id:        id,
		stopError: stopError,
	}
}

func (f *FailingStopTask) Identifier() string {
	return f.id
}

func (f *FailingStopTask) Start(ctx context.Context) error {
	f.started.Store(true)
	<-ctx.Done()
	return ctx.Err()
}

func (f *FailingStopTask) Stop(ctx context.Context) error {
	f.stopped.Store(true)
	return f.stopError
}

func (f *FailingStopTask) IsStarted() bool {
	return f.started.Load()
}

func (f *FailingStopTask) IsStopped() bool {
	return f.stopped.Load()
}

// SlowStopTask simulates a task that takes time to stop.
type SlowStopTask struct {
	id       string
	stopTime time.Duration
	started  atomic.Bool
	stopped  atomic.Bool
}

func NewSlowStopTask(id string, stopTime time.Duration) *SlowStopTask {
	return &SlowStopTask{
		id:       id,
		stopTime: stopTime,
	}
}

func (s *SlowStopTask) Identifier() string {
	return s.id
}

func (s *SlowStopTask) Start(ctx context.Context) error {
	s.started.Store(true)
	<-ctx.Done()
	return ctx.Err()
}

func (s *SlowStopTask) Stop(ctx context.Context) error {
	time.Sleep(s.stopTime)
	s.stopped.Store(true)
	return nil
}

func (s *SlowStopTask) IsStarted() bool {
	return s.started.Load()
}

func (s *SlowStopTask) IsStopped() bool {
	return s.stopped.Load()
}

// ContextAwareTask simulates a task that properly respects context cancellation.
type ContextAwareTask struct {
	id      string
	started atomic.Bool
	stopped atomic.Bool
}

func NewContextAwareTask(id string) *ContextAwareTask {
	return &ContextAwareTask{id: id}
}

func (c *ContextAwareTask) Identifier() string {
	return c.id
}

func (c *ContextAwareTask) Start(ctx context.Context) error {
	c.started.Store(true)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Simulate work
		}
	}
}

func (c *ContextAwareTask) Stop(ctx context.Context) error {
	c.stopped.Store(true)
	return nil
}

func (c *ContextAwareTask) IsStarted() bool {
	return c.started.Load()
}

func (c *ContextAwareTask) IsStopped() bool {
	return c.stopped.Load()
}
