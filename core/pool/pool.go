package pool

import "context"

// Pool defines the basic interface for a generic object pool.
type Pool[T any] interface {
	// Get retrieves an object from the pool.
	Get() T
	// Put attempts to return an object to the pool.
	Put(T) bool
}

// BlockingPool extends the Pool interface with context-aware blocking acquisition.
type BlockingPool[T any] interface {
	Pool[T]
	// GetContext waits to retrieve an object until ctx is cancelled or times out.
	GetContext(ctx context.Context) (T, bool)
}
