package tasktest

import (
	"context"
	"sync"
	"time"
)

// Mode selects how Fake.Start behaves when StartFunc is nil.
type Mode int

const (
	// ModeRunLoop blocks Start until the context is cancelled or Stop is
	// called. This matches online / captcha / scheduler: either signal unblocks
	// the run loop.
	ModeRunLoop Mode = iota
	// ModeServer blocks Start until Stop, ignoring context cancellation. This
	// matches HTTP ListenAndServe: only closing the listener returns.
	ModeServer
	// ModeOneshot returns from Start immediately with StartErr. Stop still
	// runs afterwards when the task is in a Group or Manager — cleanup is not
	// optional just because Start finished.
	ModeOneshot
)

// Fake is a test double for task.Task. Construct it with NewFake (the zero
// value is not usable), then set the exported fields before the runner calls
// Start; they are read without locking, so changing them concurrently with
// Start or Stop is a data race. The counters and signal channels are safe for
// concurrent use.
type Fake struct {
	// ID is returned by Identifier; empty means "fake".
	ID string
	// Mode selects Start's behaviour when StartFunc is nil.
	Mode Mode
	// StartErr is returned by Start in every Mode once Start unblocks (except
	// ModeRunLoop cancelled by ctx, which returns ctx.Err()).
	StartErr error
	// StopErr is returned by every Stop call when StopFunc is nil.
	StopErr error
	// StopDelay, when positive, makes the first Stop sleep before running
	// StopFunc or returning StopErr; Start is already unblocked by then.
	StopDelay time.Duration
	// StartFunc, when set, replaces Mode-driven behaviour: Start returns its
	// result. Stop does not unblock StartFunc; it must return on its own or
	// when ctx is cancelled.
	StartFunc func(context.Context) error
	// StopFunc, when set, runs once on the first Stop; its result is returned
	// by that and every later Stop call. It replaces StopErr.
	StopFunc func(context.Context) error

	mu           sync.Mutex
	startCount   int
	stopCount    int
	savedStopErr error

	startedCh    chan struct{}
	returnedCh   chan struct{}
	stoppedCh    chan struct{}
	stopCh       chan struct{}
	startedOnce  sync.Once
	returnedOnce sync.Once
	stoppedOnce  sync.Once
	stopOnce     sync.Once
}

// NewFake returns a ModeRunLoop Fake named id.
func NewFake(id string) *Fake {
	return &Fake{
		ID:         id,
		startedCh:  make(chan struct{}),
		returnedCh: make(chan struct{}),
		stoppedCh:  make(chan struct{}),
		stopCh:     make(chan struct{}),
	}
}

// Identifier returns ID, or "fake" when ID is empty.
func (f *Fake) Identifier() string {
	if f.ID == "" {
		return "fake"
	}
	return f.ID
}

// Start increments StartCount and follows Mode, StartFunc, and StartErr.
func (f *Fake) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	f.mu.Lock()
	f.startCount++
	f.mu.Unlock()
	f.startedOnce.Do(func() { close(f.startedCh) })
	defer f.returnedOnce.Do(func() { close(f.returnedCh) })

	if f.StartFunc != nil {
		return f.StartFunc(ctx)
	}
	switch f.Mode {
	case ModeOneshot:
		return f.StartErr
	case ModeServer:
		<-f.stopCh
		return f.StartErr
	default:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-f.stopCh:
			return f.StartErr
		}
	}
}

// Stop is idempotent on the hook: StopFunc/StopErr run once, later calls
// return the saved error. It always closes stopCh on first entry, unblocking
// ModeRunLoop and ModeServer.
func (f *Fake) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	f.mu.Lock()
	f.stopCount++
	f.mu.Unlock()
	f.stoppedOnce.Do(func() { close(f.stoppedCh) })

	f.stopOnce.Do(func() {
		close(f.stopCh)
		if f.StopDelay > 0 {
			time.Sleep(f.StopDelay)
		}
		var err error
		if f.StopFunc != nil {
			err = f.StopFunc(ctx)
		} else {
			err = f.StopErr
		}
		f.mu.Lock()
		f.savedStopErr = err
		f.mu.Unlock()
	})

	f.mu.Lock()
	defer f.mu.Unlock()
	return f.savedStopErr
}

// Started is closed when Start is first entered.
func (f *Fake) Started() <-chan struct{} { return f.startedCh }

// Returned is closed when Start returns.
func (f *Fake) Returned() <-chan struct{} { return f.returnedCh }

// Stopped is closed when Stop is first entered, not when StopFunc returns.
func (f *Fake) Stopped() <-chan struct{} { return f.stoppedCh }

// StartCount is how many times Start has been entered.
func (f *Fake) StartCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCount
}

// StopCount is how many times Stop has been entered.
func (f *Fake) StopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopCount
}

// IsStarted reports whether Start has been entered at least once.
func (f *Fake) IsStarted() bool { return f.StartCount() > 0 }

// IsStopped reports whether Stop has been entered (stoppedCh closed), even
// if StopFunc is still running.
func (f *Fake) IsStopped() bool {
	select {
	case <-f.stoppedCh:
		return true
	default:
		return false
	}
}
