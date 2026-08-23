// Package scripttask is a callback-backed task.Task for scripts and tests.
//
// NewScriptTask(id, onStart, onStop): Start runs onStart, or blocks on
// ctx.Done when onStart is nil. Stop runs onStop; it does not cancel Start's
// context. Direct Start+Stop with a nil onStart therefore deadlocks unless
// something else cancels ctx — put the task in a Group or Manager, which
// cancel the run context. onStop is not wrapped in Once: callers must make
// it idempotent. Started and Stopped close when Start or Stop is first
// entered, not when the hook returns.
package scripttask

import (
	"context"
	"sync"
	"sync/atomic"
)

// ScriptTask is a task.Task whose Start and Stop delegate to callbacks.
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

// NewScriptTask constructs a ScriptTask. A nil onStart makes Start block
// until ctx is cancelled. A nil onStop makes Stop return nil.
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

// Identifier returns the id passed to NewScriptTask.
func (s *ScriptTask) Identifier() string {
	return s.id
}

// Start marks the task started, closes Started, then runs onStart or waits
// on ctx.Done. It does not return because Stop was called.
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

// Stop marks the task stopped, closes Stopped, then runs onStop. Repeated
// calls run onStop again.
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

// Started is closed when Start is first entered.
func (s *ScriptTask) Started() <-chan struct{} {
	return s.startedCh
}

// Stopped is closed when Stop is first entered.
func (s *ScriptTask) Stopped() <-chan struct{} {
	return s.stoppedCh
}

// IsStarted reports whether Start has been entered.
func (s *ScriptTask) IsStarted() bool {
	return s.started.Load()
}

// IsStopped reports whether Stop has been entered.
func (s *ScriptTask) IsStopped() bool {
	return s.stopped.Load()
}
