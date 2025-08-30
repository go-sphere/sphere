package log

import (
	"sync/atomic"

	"go.uber.org/zap/zapcore"
)

// BaseLogger defines the core logging interface with structured logging capabilities.
// It provides the four standard log levels with support for structured attributes.
type BaseLogger interface {
	// Debug logs a debug-level message with optional structured attributes.
	Debug(msg string, args ...any)
	// Info logs an info-level message with optional structured attributes.
	Info(msg string, args ...any)
	// Warn logs a warning-level message with optional structured attributes.
	Warn(msg string, args ...any)
	// Error logs an error-level message with optional structured attributes.
	Error(msg string, args ...any)
}

// Logger extends BaseLogger with formatted logging capabilities.
// It combines structured logging with traditional printf-style formatting.
type Logger interface {
	BaseLogger
	// Debugf logs a debug-level message using printf-style formatting.
	Debugf(format string, args ...any)
	// Infof logs an info-level message using printf-style formatting.
	Infof(format string, args ...any)
	// Warnf logs a warning-level message using printf-style formatting.
	Warnf(format string, args ...any)
	// Errorf logs an error-level message using printf-style formatting.
	Errorf(format string, args ...any)
}

var (
	std atomic.Pointer[zapLogger]
)

func init() {
	Init(NewDefaultConfig(), nil)
}

// Init initializes the global logger with the provided configuration and attributes.
// The configuration determines output destinations and formatting, while attrs are
// added to all log messages. This should be called once during application startup.
func Init(config *Config, attrs map[string]any) {
	std.Store(newZapLogger(config,
		AddCaller(),
		AddCallerSkip(2),
		WithAttrs(attrs),
		WithStackAt(zapcore.ErrorLevel),
	))
}

// Sync flushes any buffered log entries to their destinations.
// This should typically be called before application shutdown to ensure all logs are written.
func Sync() error {
	return std.Load().logger.Sync()
}

// Debug logs a debug-level message with optional structured attributes using the global logger.
func Debug(msg string, attrs ...any) {
	std.Load().Debug(msg, attrs...)
}

// Debugf logs a debug-level message using printf-style formatting with the global logger.
func Debugf(format string, args ...any) {
	std.Load().Debugf(format, args)
}

// Info logs an info-level message with optional structured attributes using the global logger.
func Info(msg string, attrs ...any) {
	std.Load().Info(msg, attrs...)
}

// Infof logs an info-level message using printf-style formatting with the global logger.
func Infof(format string, args ...any) {
	std.Load().Infof(format, args)
}

// Warn logs a warning-level message with optional structured attributes using the global logger.
func Warn(msg string, attrs ...any) {
	std.Load().Warn(msg, attrs...)
}

// Warnf logs a warning-level message using printf-style formatting with the global logger.
func Warnf(format string, args ...any) {
	std.Load().Warnf(format, args)
}

// Error logs an error-level message with optional structured attributes using the global logger.
func Error(msg string, attrs ...any) {
	std.Load().Error(msg, attrs...)
}

// Errorf logs an error-level message using printf-style formatting with the global logger.
func Errorf(format string, args ...any) {
	std.Load().Errorf(format, args)
}

// With creates a new logger instance with additional options applied to the global logger.
// The returned logger includes the caller skip adjustment for proper call site reporting.
func With(options ...Option) Logger {
	opts := make([]Option, 0, len(options)+1)
	opts = append(opts, options...)
	opts = append(opts, AddCallerSkip(-1))
	return std.Load().With(opts...)
}
