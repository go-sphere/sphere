package scripttask

import (
	"context"
	"sync"
	"sync/atomic"
)

// ScriptTask is a task.Task whose Start and Stop delegate to callbacks and
// that records whether each was entered. Create it with NewScriptTask; the
// zero value is not usable. The state accessors (Started, Stopped, IsStarted,
// IsStopped) are safe for concurrent use; the callbacks themselves are called
// without additional synchronization.
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
