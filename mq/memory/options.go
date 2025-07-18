package memory

type options struct {
	queueSize int
}

func newOptions(opts ...Option) *options {
	o := &options{
		queueSize: 100, // default queue size
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type Option func(*options)

func WithQueueSize(size int) Option {
	return func(o *options) {
		o.queueSize = size
	}
}
