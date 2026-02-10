package scripttask

import (
	"context"
	"sync"
	"sync/atomic"
)

type ScriptTask struct {
	id string

	onStart func(context.Context) error
	onStop  func(context.Context) error

	started    atomic.Bool
	stopped    atomic.Bool
	startedCh  chan struct{}
	stoppedCh  chan struct{}
	startedSig sync.Once
	stoppedSig sync.Once
}

func NewScriptTask(
	id string,
	onStart func(context.Context) error,
	onStop func(context.Context) error,
) *ScriptTask {
	return &ScriptTask{
		id:        id,
		onStart:   onStart,
		onStop:    onStop,
		startedCh: make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

func (s *ScriptTask) Identifier() string {
	return s.id
}

func (s *ScriptTask) Start(ctx context.Context) error {
	s.started.Store(true)
	s.startedSig.Do(func() {
		close(s.startedCh)
	})

	if s.onStart != nil {
		return s.onStart(ctx)
	}

	<-ctx.Done()
	return ctx.Err()
}

func (s *ScriptTask) Stop(ctx context.Context) error {
	s.stopped.Store(true)
	s.stoppedSig.Do(func() {
		close(s.stoppedCh)
	})

	if s.onStop != nil {
		return s.onStop(ctx)
	}

	return nil
}

func (s *ScriptTask) Started() <-chan struct{} {
	return s.startedCh
}

func (s *ScriptTask) Stopped() <-chan struct{} {
	return s.stoppedCh
}

func (s *ScriptTask) IsStarted() bool {
	return s.started.Load()
}

func (s *ScriptTask) IsStopped() bool {
	return s.stopped.Load()
}
