package log

import (
	"go.uber.org/zap/zapcore"
)

type FileConfig struct {
	FileName   string `json:"file_name" yaml:"file_name"`
	MaxSize    int    `json:"max_size" yaml:"max_size"`
	MaxBackups int    `json:"max_backups" yaml:"max_backups"`
	MaxAge     int    `json:"max_age" yaml:"max_age"`
}

type ConsoleConfig struct {
	Disable bool `json:"disable" yaml:"disable"`
}

type Config struct {
	File    *FileConfig    `json:"file" yaml:"file"`
	Console *ConsoleConfig `json:"console" yaml:"console"`
	Level   string         `json:"level" yaml:"level"`
}

func NewDefaultConfig() *Config {
	return &Config{
		File:    nil,
		Console: nil,
		Level:   "info",
	}
}

type AddCallerStatus int

const (
	AddCallerStatusNone AddCallerStatus = iota
	AddCallerStatusEnable
	AddCallerStatusDisable
)

type options struct {
	addCaller  AddCallerStatus
	addStackAt zapcore.Level
	callerSkip int
	attrs      map[string]any
}

type Option = func(*options)

func AddCaller() Option {
	return func(o *options) {
		o.addCaller = AddCallerStatusEnable
	}
}

func DisableCaller() Option {
	return func(o *options) {
		o.addCaller = AddCallerStatusDisable
	}
}

func WithCallerSkip(skip int) Option {
	return func(o *options) {
		o.callerSkip = skip
	}
}
func WithStackAt(level zapcore.Level) Option {
	return func(o *options) {
		o.addStackAt = level
	}
}

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
		addCaller:  AddCallerStatusNone,
		addStackAt: zapcore.InvalidLevel,
		callerSkip: 2,
		attrs:      make(map[string]any),
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}
