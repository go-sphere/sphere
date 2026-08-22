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

// Fake is a test double for task.Task. Construct it with NewFake, then set
// Mode / StartErr / StopErr / optional Funcs before the runner calls Start.
type Fake struct {
	ID        string
	Mode      Mode
	StartErr  error
	StopErr   error
	StopDelay time.Duration
	StartFunc func(context.Context) error
	StopFunc  func(context.Context) error

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

func (f *Fake) Identifier() string {
	if f.ID == "" {
		return "fake"
	}
	return f.ID
}

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

func (f *Fake) Started() <-chan struct{}  { return f.startedCh }
func (f *Fake) Returned() <-chan struct{} { return f.returnedCh }
func (f *Fake) Stopped() <-chan struct{}  { return f.stoppedCh }

func (f *Fake) StartCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCount
}

func (f *Fake) StopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopCount
}

func (f *Fake) IsStarted() bool { return f.StartCount() > 0 }

func (f *Fake) IsStopped() bool {
	select {
	case <-f.stoppedCh:
		return true
	default:
		return false
	}
}
