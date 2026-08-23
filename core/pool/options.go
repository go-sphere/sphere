package pool

// Options is the materialized configuration consumed by NewChanPool and
// NewSyncPool. Option functions fill this struct; it is not itself a function.
type Options[T any] struct {
	New    func() T
	Reset  func(T) T
	Accept func(T) bool
	Close  func(T)
	// AllowCreate controls ChanPool.GetContext: when true (the default) and New
	// is set, an empty pool creates an object instead of waiting. SyncPool
	// ignores this field.
	AllowCreate bool
}

// Option configures Options.
type Option[T any] func(*Options[T])

// WithReset sets the function applied to an object after Accept and before Put.
func WithReset[T any](reset func(T) T) Option[T] {
	return func(o *Options[T]) {
		o.Reset = reset
	}
}

// WithNew sets the factory used when Get finds the pool empty. SyncPool.Get
// returns the zero value of T when New is omitted and the pool has nothing.
func WithNew[T any](newFunc func() T) Option[T] {
	return func(o *Options[T]) {
		o.New = newFunc
	}
}

// WithAccept sets the predicate that decides whether Put retains an object.
// A rejected object is not reset, not pooled, and not passed to Close.
func WithAccept[T any](accept func(T) bool) Option[T] {
	return func(o *Options[T]) {
		o.Accept = accept
	}
}

// WithClose sets the destructor ChanPool.Close runs on objects still in the
// channel. SyncPool ignores it. Overflow Puts that fail because the channel is
// full do not call this function.
func WithClose[T any](close func(T)) Option[T] {
	return func(o *Options[T]) {
		o.Close = close
	}
}

// WithAllowCreate controls whether ChanPool.GetContext may call New when the
// pool is empty instead of waiting. Default is true. SyncPool ignores it.
func WithAllowCreate[T any](allow bool) Option[T] {
	return func(o *Options[T]) {
		o.AllowCreate = allow
	}
}

func newOptions[T any](opts ...Option[T]) *Options[T] {
	options := &Options[T]{
		AllowCreate: true, // default to true for backward compatibility
	}
	for _, o := range opts {
		o(options)
	}
	return options
}
