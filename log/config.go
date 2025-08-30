package log

import (
	"go.uber.org/zap/zapcore"
)

// FileConfig configures file-based logging output with rotation settings.
// It supports automatic log rotation based on size, age, and backup count.
type FileConfig struct {
	FileName   string `json:"file_name" yaml:"file_name"`
	MaxSize    int    `json:"max_size" yaml:"max_size"`
	MaxBackups int    `json:"max_backups" yaml:"max_backups"`
	MaxAge     int    `json:"max_age" yaml:"max_age"`
}

// ConsoleConfig configures console logging output.
// When Disable is true, console logging is completely turned off.
type ConsoleConfig struct {
	Disable bool `json:"disable" yaml:"disable"`
}

// Config defines the complete logging configuration including output destinations,
// log level, and formatting options. It supports both console and file outputs.
type Config struct {
	File    *FileConfig    `json:"file" yaml:"file"`
	Console *ConsoleConfig `json:"console" yaml:"console"`
	Level   string         `json:"level" yaml:"level"`
}

// NewDefaultConfig creates a new Config with sensible defaults.
// Returns a configuration with info level logging and no file output.
func NewDefaultConfig() *Config {
	return &Config{
		File:    nil,
		Console: nil,
		Level:   "info",
	}
}

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
	addStackAt zapcore.Level
	callerSkip int
	attrs      map[string]any
}

// Option is a function type for configuring logger options.
type Option = func(*options)

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
func WithStackAt(level zapcore.Level) Option {
	return func(o *options) {
		o.addStackAt = level
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
		addStackAt: zapcore.InvalidLevel,
		callerSkip: 0,
		attrs:      make(map[string]any),
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}
