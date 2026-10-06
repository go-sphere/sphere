package log

import (
	"context"
	"log/slog"
)

// Level is a backend-agnostic log level. Higher values are more severe;
// filters compare with < and >=.
type Level int8

// Supported levels, in increasing severity.
const (
	LevelDebug Level = iota // debug
	LevelInfo               // info
	LevelWarn               // warn
	LevelError              // error
)

// Backend is the pluggable logging backend behind every Logger.
//
// Log writes one entry; implementations should be safe for concurrent use
// because the global logger is shared across goroutines. Sync flushes
// buffered entries. With returns a derived backend carrying the options; the
// built-in backends leave the receiver unchanged. Close is not part of this
// interface; type-assert [io.Closer] when the backend owns a handle.
type Backend interface {
	Log(ctx context.Context, level Level, msg string, attrs ...Attr)
	Sync() error
	With(options ...Option) Backend
}

// SlogBackend is an optional capability for backends that can expose a
// *slog.Logger. boot.WithLoggerBackend type-asserts it and, when present,
// installs SlogLogger() as the default slog logger so the standard library's
// slog output goes through the same backend. zapx.Backend implements it.
type SlogBackend interface {
	SlogLogger(options ...Option) *slog.Logger
}
