package pool

import "sync"

// SyncPool is an unbounded pool backed by sync.Pool. WithClose and
// WithAllowCreate are ignored. Objects are GC-eligible when idle.
type SyncPool[T any] struct {
	p          sync.Pool
	resetFunc  func(T) T
	acceptFunc func(T) bool
}

// NewSyncPool creates a SyncPool. Omit WithNew and Get may return the zero value of T.
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

// Get returns a pooled object or the result of New. Without WithNew, an empty
// pool yields the zero value of T.
func (sp *SyncPool[T]) Get() T {
	obj := sp.p.Get()
	if obj != nil {
		return obj.(T)
	}
	var zero T
	return zero
}

// Put retains obj if Accept allows it. False means it was discarded without Reset.
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
