package log

import (
	"sync"

	"go.uber.org/zap/zapcore"
)

type BaseLogger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Logger interface {
	BaseLogger
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

var (
	mu  sync.Mutex
	std *zapLogger
)

func init() {
	Init(NewDefaultConfig(), nil)
}

func Init(opts *Config, attrs map[string]any) {
	mu.Lock()
	defer mu.Unlock()
	std = newZapLogger(opts,
		WithAttrs(attrs),
		AddCaller(),
		AddCallerSkip(2),
		WithStackAt(zapcore.ErrorLevel),
	)
}

func Sync() error {
	return std.logger.Sync()
}

func Debug(msg string, attrs ...any) {
	std.Debug(msg, attrs...)
}

func Debugf(format string, args ...any) {
	std.Debugf(format, args)
}

func Info(msg string, attrs ...any) {
	std.Info(msg, attrs...)
}

func Infof(format string, args ...any) {
	std.Infof(format, args)
}

func Warn(msg string, attrs ...any) {
	std.Warn(msg, attrs...)
}

func Warnf(format string, args ...any) {
	std.Warnf(format, args)
}

func Error(msg string, attrs ...any) {
	std.Error(msg, attrs...)
}

func Errorf(format string, args ...any) {
	std.Errorf(format, args)
}

func With(options ...Option) Logger {
	opts := make([]Option, 0, len(options)+1)
	opts = append(opts, options...)
	opts = append(opts, AddCallerSkip(-1))
	return std.With(opts...)
}
