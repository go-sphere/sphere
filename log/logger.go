package log

import (
	"context"
	"fmt"
	"sync/atomic"
)

// BaseLogger defines structured logging methods.
type BaseLogger interface {
	Debug(msg string, attrs ...Attr)
	Info(msg string, attrs ...Attr)
	Warn(msg string, attrs ...Attr)
	Error(msg string, attrs ...Attr)
}

// ContextLogger defines context-aware structured logging methods.
type ContextLogger interface {
	DebugContext(ctx context.Context, msg string, attrs ...Attr)
	InfoContext(ctx context.Context, msg string, attrs ...Attr)
	WarnContext(ctx context.Context, msg string, attrs ...Attr)
	ErrorContext(ctx context.Context, msg string, attrs ...Attr)
}

// FormatLogger defines printf-style logging methods.
type FormatLogger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Logger combines quick-use APIs with type-safe attrs and context support.
type Logger interface {
	BaseLogger
	ContextLogger
	FormatLogger

	Backend() Backend
	With(options ...Option) Logger
	Sync() error
}

type coreLogger struct {
	backend Backend
}

// Backend returns the underlying backend for bridge adapters.
func (l *coreLogger) Backend() Backend {
	return l.backend
}

func (l *coreLogger) Debug(msg string, attrs ...Attr) {
	l.backend.Log(context.Background(), LevelDebug, msg, attrs...)
}

func (l *coreLogger) Info(msg string, attrs ...Attr) {
	l.backend.Log(context.Background(), LevelInfo, msg, attrs...)
}

func (l *coreLogger) Warn(msg string, attrs ...Attr) {
	l.backend.Log(context.Background(), LevelWarn, msg, attrs...)
}

func (l *coreLogger) Error(msg string, attrs ...Attr) {
	l.backend.Log(context.Background(), LevelError, msg, attrs...)
}

func (l *coreLogger) DebugContext(ctx context.Context, msg string, attrs ...Attr) {
	l.backend.Log(ctx, LevelDebug, msg, attrs...)
}

func (l *coreLogger) InfoContext(ctx context.Context, msg string, attrs ...Attr) {
	l.backend.Log(ctx, LevelInfo, msg, attrs...)
}

func (l *coreLogger) WarnContext(ctx context.Context, msg string, attrs ...Attr) {
	l.backend.Log(ctx, LevelWarn, msg, attrs...)
}

func (l *coreLogger) ErrorContext(ctx context.Context, msg string, attrs ...Attr) {
	l.backend.Log(ctx, LevelError, msg, attrs...)
}

func (l *coreLogger) Debugf(format string, args ...any) {
	l.backend.Log(context.Background(), LevelDebug, fmt.Sprintf(format, args...))
}

func (l *coreLogger) Infof(format string, args ...any) {
	l.backend.Log(context.Background(), LevelInfo, fmt.Sprintf(format, args...))
}

func (l *coreLogger) Warnf(format string, args ...any) {
	l.backend.Log(context.Background(), LevelWarn, fmt.Sprintf(format, args...))
}

func (l *coreLogger) Errorf(format string, args ...any) {
	l.backend.Log(context.Background(), LevelError, fmt.Sprintf(format, args...))
}

func (l *coreLogger) With(options ...Option) Logger {
	return &coreLogger{backend: l.backend.With(options...)}
}

func (l *coreLogger) Sync() error {
	return l.backend.Sync()
}

var std atomic.Pointer[coreLogger]

func init() {
	std.Store(&coreLogger{backend: &StdioBackend{}})
}

// InitWithBackends initializes global logger with custom backend(s).
func InitWithBackends(backends ...Backend) {
	std.Store(&coreLogger{backend: NewMultiBackend(backends...)})
}

func logger() *coreLogger {
	if l := std.Load(); l != nil {
		return l
	}
	fallback := &coreLogger{backend: &StdioBackend{}}
	std.Store(fallback)
	return fallback
}

func Debug(msg string, attrs ...Attr) {
	logger().backend.Log(context.Background(), LevelDebug, msg, attrs...)
}

func DebugContext(ctx context.Context, msg string, attrs ...Attr) {
	logger().backend.Log(ctx, LevelDebug, msg, attrs...)
}

func Debugf(format string, args ...any) {
	logger().backend.Log(context.Background(), LevelDebug, fmt.Sprintf(format, args...))
}

func Info(msg string, attrs ...Attr) {
	logger().backend.Log(context.Background(), LevelInfo, msg, attrs...)
}

func InfoContext(ctx context.Context, msg string, attrs ...Attr) {
	logger().backend.Log(ctx, LevelInfo, msg, attrs...)
}

func Infof(format string, args ...any) {
	logger().backend.Log(context.Background(), LevelInfo, fmt.Sprintf(format, args...))
}

func Warn(msg string, attrs ...Attr) {
	logger().backend.Log(context.Background(), LevelWarn, msg, attrs...)
}

func WarnContext(ctx context.Context, msg string, attrs ...Attr) {
	logger().backend.Log(ctx, LevelWarn, msg, attrs...)
}

func Warnf(format string, args ...any) {
	logger().backend.Log(context.Background(), LevelWarn, fmt.Sprintf(format, args...))
}

func Error(msg string, attrs ...Attr) {
	logger().backend.Log(context.Background(), LevelError, msg, attrs...)
}

func ErrorContext(ctx context.Context, msg string, attrs ...Attr) {
	logger().backend.Log(ctx, LevelError, msg, attrs...)
}

func Errorf(format string, args ...any) {
	logger().backend.Log(context.Background(), LevelError, fmt.Sprintf(format, args...))
}

func With(options ...Option) Logger {
	return logger().With(options...)
}

func Sync() error {
	return logger().Sync()
}
