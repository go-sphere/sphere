package log

import (
	"sync/atomic"

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
	std atomic.Pointer[zapLogger]
)

func init() {
	Init(NewDefaultConfig(), nil)
}

func Init(config *Config, attrs map[string]any) {
	std.Store(newZapLogger(config,
		AddCaller(),
		AddCallerSkip(2),
		WithAttrs(attrs),
		WithStackAt(zapcore.ErrorLevel),
	))
}

func Sync() error {
	return std.Load().logger.Sync()
}

func Debug(msg string, attrs ...any) {
	std.Load().Debug(msg, attrs...)
}

func Debugf(format string, args ...any) {
	std.Load().Debugf(format, args)
}

func Info(msg string, attrs ...any) {
	std.Load().Info(msg, attrs...)
}

func Infof(format string, args ...any) {
	std.Load().Infof(format, args)
}

func Warn(msg string, attrs ...any) {
	std.Load().Warn(msg, attrs...)
}

func Warnf(format string, args ...any) {
	std.Load().Warnf(format, args)
}

func Error(msg string, attrs ...any) {
	std.Load().Error(msg, attrs...)
}

func Errorf(format string, args ...any) {
	std.Load().Errorf(format, args)
}

func With(options ...Option) Logger {
	opts := make([]Option, 0, len(options)+1)
	opts = append(opts, options...)
	opts = append(opts, AddCallerSkip(-1))
	return std.Load().With(opts...)
}
