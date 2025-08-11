package log

import (
	"fmt"
	"log/slog"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

type Logger interface {
	Debug(msg string, attrs ...any)
	Info(msg string, attrs ...any)
	Warn(msg string, attrs ...any)
	Error(msg string, attrs ...any)
}

var (
	mu      sync.Mutex
	std     Logger
	backend *zapLogger
)

func init() {
	Init(NewOptions(), []slog.Attr{})
}

func Init(opts *Options, attrs []slog.Attr) {
	mu.Lock()
	defer mu.Unlock()
	backend = newZapLogHandler(opts, attrs)
	std = slog.New(backend.Handler(
		zapslog.WithCallerSkip(1),
		zapslog.WithCaller(true),
		zapslog.AddStacktraceAt(slog.LevelError),
	))
}

func Sync() error {
	return backend.core.Sync()
}

func ZapLogger(options ...zap.Option) *zap.Logger {
	fields := make([]zap.Field, 0, len(backend.attrs))
	for _, attr := range backend.attrs {
		fields = append(fields, zap.Any(attr.Key, attr.Value.Any()))
	}
	return zap.New(backend.core, options...).With(fields...)
}

func Debug(msg string, attrs ...interface{}) {
	std.Debug(msg, attrs...)
}

func Debugf(format string, args ...interface{}) {
	std.Debug(fmt.Sprintf(format, args...))
}

func Info(msg string, attrs ...interface{}) {
	std.Info(msg, attrs...)
}

func Infof(format string, args ...interface{}) {
	std.Info(fmt.Sprintf(format, args...))
}

func Warn(msg string, attrs ...interface{}) {
	std.Warn(msg, attrs...)
}

func Warnf(format string, args ...interface{}) {
	std.Warn(fmt.Sprintf(format, args...))
}

func Error(msg string, attrs ...interface{}) {
	std.Error(msg, attrs...)
}

func Errorf(format string, args ...interface{}) {
	std.Error(fmt.Sprintf(format, args...))
}
