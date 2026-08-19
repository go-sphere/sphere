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
//
// Sizes below 1 are ignored and the default is kept. A negative size panicked
// on the first Publish or Subscribe, far from the configuration that caused it.
// Zero was worse: it produced an unbuffered channel, and because PubSub
// broadcasts with a non-blocking send, delivery then depended on a subscriber
// happening to be parked in its receive at that instant — an idle, perfectly
// healthy subscriber dropped most of its messages, while the drop-on-full design
// is meant to shed load only for a subscriber that has fallen behind.
func WithQueueSize(size int) Option {
	return func(o *options) {
		if size < 1 {
			return
		}
		o.queueSize = size
	}
}
