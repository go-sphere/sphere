package pool

import (
	"context"
	"sync"
)

// ChanPool is a bounded channel-backed pool. Objects stay reachable until Get
// or Close; overflow Puts drop the object without calling Close.
type ChanPool[T any] struct {
	ch          chan T
	newFn       func() T
	reset       func(T) T
	accept      func(T) bool
	closeFn     func(T)
	allowCreate bool
	mu          sync.RWMutex
	closed      bool
}

// NewChanPool creates a ChanPool of the given capacity. A size of 0 becomes 1.
func NewChanPool[T any](size int, opts ...Option[T]) *ChanPool[T] {
	if size <= 0 {
		size = 1
	}
	options := newOptions(opts...)
	return &ChanPool[T]{
		ch:          make(chan T, size),
		newFn:       options.New,
		reset:       options.Reset,
		accept:      options.Accept,
		closeFn:     options.Close,
		allowCreate: options.AllowCreate,
	}
}

// Get takes an object if one is immediately available; otherwise it calls New
// or returns the zero value. After Close, leftover items can still be received
// when no Close callback was set to drain them.
func (cp *ChanPool[T]) Get() T {
	select {
	case obj := <-cp.ch:
		return obj
	default:
		if cp.newFn != nil {
			return cp.newFn()
		}
		var zero T
		return zero
	}
}

// Put retains obj if Accept allows it, Reset has run, and the pool is neither
// closed nor full. False means the object was not pooled and Close was not called.
func (cp *ChanPool[T]) Put(obj T) bool {
	if cp.accept != nil && !cp.accept(obj) {
		return false
	}
	if cp.reset != nil {
		obj = cp.reset(obj)
	}
	// Hold the read lock so that closed is observed and the send happens
	// atomically with respect to Close, preventing a send on a closed channel.
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	if cp.closed {
		return false
	}
	select {
	case cp.ch <- obj:
		return true
	default:
		return false
	}
}

// GetContext returns (zero, false) if the pool is already closed, closed while
// waiting, or ctx is done. An immediately available item is taken without
// creating. Otherwise, when AllowCreate and New are set, a new object is
// created; if not, GetContext waits on the channel.
func (cp *ChanPool[T]) GetContext(ctx context.Context) (T, bool) {
	if cp.IsClosed() {
		var zero T
		return zero, false
	}

	select {
	case obj, ok := <-cp.ch:
		if !ok {
			var zero T
			return zero, false
		}
		return obj, true
	default:
	}

	// Only create new object if explicitly allowed
	if cp.allowCreate && cp.newFn != nil {
		return cp.newFn(), true
	}

	// Wait for object from pool or context cancellation. If the pool is closed
	// while blocking here, the receive fails and reports (zero, false).
	select {
	case obj, ok := <-cp.ch:
		if !ok {
			var zero T
			return zero, false
		}
		return obj, true
	case <-ctx.Done():
		var zero T
		return zero, false
	}
}

// Len returns the number of objects currently cached in the pool.
func (cp *ChanPool[T]) Len() int {
	return len(cp.ch)
}

// Cap returns the capacity of the pool.
func (cp *ChanPool[T]) Cap() int {
	return cap(cp.ch)
}

// Close is idempotent. GetContext afterwards returns (zero, false) even if
// items remain. If a Close callback is set, remaining items are drained and
// passed to it; otherwise they stay in the closed channel for a later Get.
func (cp *ChanPool[T]) Close() {
	cp.mu.Lock()
	if cp.closed {
		cp.mu.Unlock()
		return // already closed
	}
	cp.closed = true
	close(cp.ch)
	cp.mu.Unlock()

	// Safe to drain without the lock: closed is set, so concurrent Put calls
	// observe it and never send on the closed channel.
	if cp.closeFn != nil {
		for obj := range cp.ch {
			cp.closeFn(obj)
		}
	}
}

// IsClosed returns whether the pool has been closed.
func (cp *ChanPool[T]) IsClosed() bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.closed
}
