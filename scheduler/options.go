package scheduler

import "time"

// EnqueueOptions is the materialized set consumed by Producer.Enqueue.
// Drivers apply only the fields they support; the asynq driver forwards only
// MaxRetry > 0, so a value <= 0 leaves asynq's default (typically 25) in place
// rather than disabling retries.
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

// WithMaxRetry sets the retry budget. The asynq driver forwards only values
// > 0, so WithMaxRetry(0) (or a negative value) leaves asynq's default retry
// count in place instead of disabling retries.
func WithMaxRetry(n int) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.MaxRetry = n
	}
}

// WithQueue selects the queue used by Enqueue; it must be one of the queues
// configured on the Scheduler. Periodic tasks are unaffected and always run on
// the "default" queue.
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
