package pool

import "sync"

// SyncPool is a generic object pool based on sync.Pool, suitable for frequent allocation of small objects.
type SyncPool[T any] struct {
	p          sync.Pool
	resetFunc  func(T) T
	acceptFunc func(T) bool
}

// NewSyncPool creates an object pool based on sync.Pool.
func NewSyncPool[T any](opts ...Option[T]) *SyncPool[T] {
	options := newOptions(opts...)
	sp := &SyncPool[T]{
		resetFunc:  options.Reset,
		acceptFunc: options.Accept,
	}
	if options.New != nil {
		newFn := options.New
		sp.p.New = func() any {
			return newFn()
		}
	}
	return sp
}

// Get returns an object from the pool or creates a new one via the New function.
func (sp *SyncPool[T]) Get() T {
	obj := sp.p.Get()
	if obj != nil {
		return obj.(T)
	}
	var zero T
	return zero
}

// Put returns an object to the pool.
func (sp *SyncPool[T]) Put(obj T) bool {
	if sp.acceptFunc != nil && !sp.acceptFunc(obj) {
		return false
	}
	if sp.resetFunc != nil {
		obj = sp.resetFunc(obj)
	}
	sp.p.Put(obj)
	return true
}
