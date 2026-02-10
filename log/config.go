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

type options struct {
	name       string
	addCaller  AddCallerStatus
	addStackAt *Level
	callerSkip int
	attrs      map[string]any
}

// Option is a function type for configuring logger options.
type Option = func(*options)

// ResolvedOptions is a materialized Option set for backend adapters.
type ResolvedOptions struct {
	Name       string
	AddCaller  AddCallerStatus
	AddStackAt *Level
	CallerSkip int
	Attrs      map[string]any
}

// WithName sets the logger name for identification purposes.
// The name appears in log output to help distinguish between different loggers.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// AddCaller enables caller information in log entries.
// This includes file names and line numbers where the log call was made.
func AddCaller() Option {
	return func(o *options) {
		o.addCaller = AddCallerStatusEnable
	}
}

// DisableCaller removes caller information from log entries.
// This can improve performance when caller information is not needed.
func DisableCaller() Option {
	return func(o *options) {
		o.addCaller = AddCallerStatusDisable
	}
}

// AddCallerSkip adjusts the caller skip count for accurate call site reporting.
// This is useful when wrapping the logger to ensure the correct caller is reported.
func AddCallerSkip(skip int) Option {
	return func(o *options) {
		o.callerSkip += skip
	}
}

// WithStackAt enables stack trace logging at the specified level and above.
// Stack traces help debug issues by showing the full call chain.
func WithStackAt(level Level) Option {
	return func(o *options) {
		l := level
		o.addStackAt = &l
	}
}

// WithAttrs adds structured attributes to all log messages from this logger.
// These attributes provide consistent context across all log entries.
func WithAttrs(attrs map[string]any) Option {
	return func(o *options) {
		if attrs != nil {
			if o.attrs == nil {
				o.attrs = make(map[string]any)
			}
			for k, v := range attrs {
				o.attrs[k] = v
			}
		}
	}
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		addCaller:  AddCallerStatusKeep,
		addStackAt: nil,
		callerSkip: 0,
		attrs:      make(map[string]any),
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// ResolveOptions materializes options so adapter packages can consume them.
func ResolveOptions(opts ...Option) ResolvedOptions {
	o := newOptions(opts...)
	attrs := make(map[string]any, len(o.attrs))
	for k, v := range o.attrs {
		attrs[k] = v
	}
	var stackAt *Level
	if o.addStackAt != nil {
		l := *o.addStackAt
		stackAt = &l
	}
	return ResolvedOptions{
		Name:       o.name,
		AddCaller:  o.addCaller,
		AddStackAt: stackAt,
		CallerSkip: o.callerSkip,
		Attrs:      attrs,
	}
}
