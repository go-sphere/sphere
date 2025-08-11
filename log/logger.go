package log

import (
	"sync"

	"go.uber.org/zap"
)

type Logger interface {
	Debug(msg string, attrs ...any)
	Info(msg string, attrs ...any)
	Warn(msg string, attrs ...any)
	Error(msg string, attrs ...any)

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
	std = newZapLogger(opts, WithAttrs(attrs))
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

func With(attrs map[string]any) Logger {
	fields := make([]any, 0, len(attrs))
	for k, v := range attrs {
		fields = append(fields, zap.Any(k, v))
	}
	return &zapLogger{
		logger: std.logger.With(fields...),
	}
}
