package log

import (
	"sync"

	"github.com/TBXark/sphere/pkg/log/logfields"
	"go.uber.org/zap"
)

// Logger is a contract for the logger
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Debugw(message string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Infow(message string, args ...interface{})

	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Warnw(message string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Errorw(message string, args ...interface{})

	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	Panicw(message string, args ...interface{})

	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Fatalw(message string, args ...interface{})

	WithFields(keyValues map[string]interface{}) Logger
}

var (
	mu  sync.Mutex
	std = newLogger(NewOptions())
)

func Init(opts *Options, fields ...logfields.Field) {
	mu.Lock()
	defer mu.Unlock()
	std = newLogger(opts, fields...)
}

func NewLogger(opts *Options) Logger {
	return newLogger(opts)
}

func Sync() error {
	return std.sugarLogger.Sync()
}

func DisableCaller() Logger {
	mu.Lock()
	defer mu.Unlock()
	return &zapLogger{std.sugarLogger.WithOptions(zap.WithCaller(false))}
}

func ZapLogger() *zap.Logger {
	mu.Lock()
	defer mu.Unlock()
	return std.sugarLogger.Desugar().WithOptions(zap.WithCaller(false))
}

// Logger implementation

func Debug(args ...interface{}) {
	std.Debug(args...)
}

func Info(args ...interface{}) {
	std.Info(args...)
}

func Warn(args ...interface{}) {
	std.Warn(args...)
}

func Error(args ...interface{}) {
	std.Error(args...)
}

func Panic(args ...interface{}) {
	std.Panic(args...)
}

func Fatal(args ...interface{}) {
	std.Fatal(args...)
}

func Debugf(format string, args ...interface{}) {
	std.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	std.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	std.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	std.Errorf(format, args...)
}

func Panicf(format string, args ...interface{}) {
	std.Panicf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	std.Fatalf(format, args...)
}

func Debugw(message string, args ...interface{}) {
	std.Debugw(message, args...)
}

func Infow(message string, args ...interface{}) {
	std.Infow(message, args...)
}

func Warnw(message string, args ...interface{}) {
	std.Warnw(message, args...)
}

func Errorw(message string, args ...interface{}) {
	std.Errorw(message, args...)
}

func Panicw(message string, args ...interface{}) {
	std.Panicw(message, args...)
}

func Fatalw(message string, args ...interface{}) {
	std.Fatalw(message, args...)
}

func WithFields(fields map[string]interface{}) Logger {
	if len(fields) == 0 {
		return std
	}
	return std.WithFields(fields)
}
