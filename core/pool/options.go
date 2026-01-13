package pool

// Options contains configuration functions for pool behavior.
type Options[T any] struct {
	New         func() T
	Reset       func(T) T
	Accept      func(T) bool
	Close       func(T)
	AllowCreate bool // For ChanPool.GetContext: if true, create new object when pool is empty
}

// Option is a function that configures Options.
type Option[T any] func(*Options[T])

// WithReset sets the reset function for pool objects.
func WithReset[T any](reset func(T) T) Option[T] {
	return func(o *Options[T]) {
		o.Reset = reset
	}
}

func WithNew[T any](newFunc func() T) Option[T] {
	return func(o *Options[T]) {
		o.New = newFunc
	}
}

func WithAccept[T any](accept func(T) bool) Option[T] {
	return func(o *Options[T]) {
		o.Accept = accept
	}
}

func WithClose[T any](close func(T)) Option[T] {
	return func(o *Options[T]) {
		o.Close = close
	}
}

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
