package scheduler

import "time"

type EnqueueOptions struct {
	Delay     time.Duration
	Deadline  time.Time
	MaxRetry  int
	Queue     string
	UniqueFor time.Duration
	Retention time.Duration
	TaskID    string
}

// EnqueueOption customizes Producer.Enqueue.
type EnqueueOption func(*EnqueueOptions)

func WithDelay(d time.Duration) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.Delay = d
	}
}

func WithDeadline(t time.Time) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.Deadline = t
	}
}

func WithMaxRetry(n int) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.MaxRetry = n
	}
}

func WithQueue(name string) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.Queue = name
	}
}

func WithUniqueFor(d time.Duration) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.UniqueFor = d
	}
}

func WithRetention(d time.Duration) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.Retention = d
	}
}

func WithTaskID(id string) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.TaskID = id
	}
}

func ApplyEnqueueOptions(opts ...EnqueueOption) EnqueueOptions {
	applied := EnqueueOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&applied)
		}
	}
	return applied
}
