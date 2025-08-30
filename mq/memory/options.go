package memory

// options holds configuration parameters for memory-based message queue implementations.
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

// Option defines a function type for configuring memory message queue options.
type Option func(*options)

// WithQueueSize sets the buffer size for message channels in the memory queue.
// A larger size allows more messages to be buffered before blocking publishers.
func WithQueueSize(size int) Option {
	return func(o *options) {
		o.queueSize = size
	}
}
