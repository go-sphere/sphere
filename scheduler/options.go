package scheduler

import "time"

// EnqueueOptions is the materialized set consumed by Producer.Enqueue.
// Drivers apply only the fields they support; asynq ignores MaxRetry <= 0
// (asynq's own default, typically 25, remains).
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

// WithDelay postpones execution by d after Enqueue.
func WithDelay(d time.Duration) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.Delay = d
	}
}

// WithDeadline sets an absolute deadline after which the task is not run.
func WithDeadline(t time.Time) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.Deadline = t
	}
}

// WithMaxRetry sets the retry budget. Values <= 0 are ignored by the asynq
// driver, which then uses asynq's default rather than zero retries.
func WithMaxRetry(n int) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.MaxRetry = n
	}
}

// WithQueue selects the asynq queue name. Periodic tasks require "default".
func WithQueue(name string) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.Queue = name
	}
}

// WithUniqueFor deduplicates tasks of the same kind for duration d.
func WithUniqueFor(d time.Duration) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.UniqueFor = d
	}
}

// WithRetention keeps the task record for d after it reaches a terminal state.
func WithRetention(d time.Duration) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.Retention = d
	}
}

// WithTaskID sets an explicit task id (asynq uniqueness / idempotency key).
func WithTaskID(id string) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.TaskID = id
	}
}

// ApplyEnqueueOptions folds opts into an EnqueueOptions value. Nil options are skipped.
func ApplyEnqueueOptions(opts ...EnqueueOption) EnqueueOptions {
	applied := EnqueueOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&applied)
		}
	}
	return applied
}
