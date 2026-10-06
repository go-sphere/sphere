package pool

import "context"

// Pool is a generic object pool. Get returns a pooled or newly created object
// (or the zero value of T when none is available and no New is configured).
// Put reports whether the object was retained; a false result means the pool
// dropped it and ownership stays with the caller.
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
