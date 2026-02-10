package log

// AddCallerStatus represents the state of caller information in log entries.
type AddCallerStatus int

const (
	// AddCallerStatusKeep maintains the current caller setting without changes.
	AddCallerStatusKeep AddCallerStatus = iota
	// AddCallerStatusEnable adds caller information to log entries.
	AddCallerStatusEnable
	// AddCallerStatusDisable removes caller information from log entries.
	AddCallerStatusDisable
)

// Options is the materialized option set consumed by backend adapters.
type Options struct {
	Name       string
	AddCaller  AddCallerStatus
	AddStackAt *Level
	Attrs      map[string]any
}

// Option is a function type for configuring logger options.
type Option = func(*Options)

// WithName sets the logger name for identification purposes.
// The name appears in log output to help distinguish between different loggers.
func WithName(name string) Option {
	return func(o *Options) {
		o.Name = name
	}
}

// AddCaller enables caller information in log entries.
// This includes file names and line numbers where the log call was made.
func AddCaller() Option {
	return func(o *Options) {
		o.AddCaller = AddCallerStatusEnable
	}
}

// DisableCaller removes caller information from log entries.
// This can improve performance when caller information is not needed.
func DisableCaller() Option {
	return func(o *Options) {
		o.AddCaller = AddCallerStatusDisable
	}
}

// WithStackAt enables stack trace logging at the specified level and above.
// Stack traces help debug issues by showing the full call chain.
func WithStackAt(level Level) Option {
	return func(o *Options) {
		l := level
		o.AddStackAt = &l
	}
}

// WithAttrs adds structured attributes to all log messages from this logger.
// These attributes provide consistent context across all log entries.
func WithAttrs(attrs map[string]any) Option {
	return func(o *Options) {
		if attrs != nil {
			if o.Attrs == nil {
				o.Attrs = make(map[string]any)
			}
			for k, v := range attrs {
				o.Attrs[k] = v
			}
		}
	}
}

func newOptions(opts ...Option) *Options {
	defaults := &Options{
		AddCaller:  AddCallerStatusKeep,
		AddStackAt: nil,
		Attrs:      make(map[string]any),
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// NewOptions materializes options so backend adapters can consume them.
func NewOptions(opts ...Option) Options {
	o := newOptions(opts...)
	attrs := make(map[string]any, len(o.Attrs))
	for k, v := range o.Attrs {
		attrs[k] = v
	}
	var stackAt *Level
	if o.AddStackAt != nil {
		l := *o.AddStackAt
		stackAt = &l
	}
	return Options{
		Name:       o.Name,
		AddCaller:  o.AddCaller,
		AddStackAt: stackAt,
		Attrs:      attrs,
	}
}
