// Package pool offers two generic object pools that share the same Option set.
//
// SyncPool wraps sync.Pool: unbounded, GC-eligible, no Close. Use it for
// short-lived buffers. ChanPool is a buffered channel with a capacity: use it
// for connections or file handles that must be closed. A size of 0 becomes 1.
// WithClose and WithAllowCreate are ChanPool-only; SyncPool ignores them.
//
// Put runs Accept then Reset; a rejected object is neither reset, pooled, nor
// passed to Close. GetContext on a closed ChanPool returns (zero, false) even
// if items remain in the channel. After Close, Get still drains leftover items
// when no Close callback was set.
package pool

import "context"

// Pool is a generic object pool. Put reports whether the object was retained.
type Pool[T any] interface {
	Get() T
	Put(T) bool
}

// BlockingPool adds a context-aware acquire. The bool is false when ctx is
// done, the pool is closed, or (for ChanPool) the channel is already closed.
type BlockingPool[T any] interface {
	Pool[T]
	GetContext(ctx context.Context) (T, bool)
}
