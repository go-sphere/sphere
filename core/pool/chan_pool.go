package pool

import (
	"context"
	"sync/atomic"
)

// ChanPool is a bounded object pool based on channels, suitable for scenarios requiring resource limits.
// Unlike SyncPool, ChanPool has an explicit capacity limit and objects are not subject to GC.
type ChanPool[T any] struct {
	ch          chan T
	newFn       func() T
	reset       func(T) T
	accept      func(T) bool
	closeFn     func(T)
	allowCreate bool
	closed      atomic.Bool
}

// NewChanPool creates a bounded object pool based on channels.
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

// Get retrieves an object from the pool without blocking.
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

// Put attempts to return an object to the pool.
func (cp *ChanPool[T]) Put(obj T) bool {
	if cp.accept != nil && !cp.accept(obj) {
		return false
	}
	if cp.reset != nil {
		obj = cp.reset(obj)
	}
	select {
	case cp.ch <- obj:
		return true
	default:
		return false
	}
}

// GetContext waits to retrieve an object until ctx is cancelled or times out.
// If allowCreate is true and newFn is set, creates a new object when pool is empty.
// If allowCreate is false, always waits for an object from the pool.
func (cp *ChanPool[T]) GetContext(ctx context.Context) (T, bool) {
	if cp.closed.Load() {
		var zero T
		return zero, false
	}

	select {
	case obj := <-cp.ch:
		return obj, true
	default:
	}

	// Only create new object if explicitly allowed
	if cp.allowCreate && cp.newFn != nil {
		return cp.newFn(), true
	}

	// Wait for object from pool or context cancellation
	select {
	case obj := <-cp.ch:
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

// Close closes the pool and calls closeFn on all remaining objects.
// After calling Close, Get and GetContext will return zero values.
func (cp *ChanPool[T]) Close() {
	if !cp.closed.CompareAndSwap(false, true) {
		return // already closed
	}

	close(cp.ch)

	if cp.closeFn != nil {
		for obj := range cp.ch {
			cp.closeFn(obj)
		}
	}
}

// IsClosed returns whether the pool has been closed.
func (cp *ChanPool[T]) IsClosed() bool {
	return cp.closed.Load()
}
