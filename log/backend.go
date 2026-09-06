package log

import (
	"context"
	"log/slog"
)

// Level is a backend-agnostic log level.
type Level int8

const (
	LevelDebug Level = iota // debug
	LevelInfo               // info
	LevelWarn               // warn
	LevelError              // error
)

// Backend is the pluggable logging backend. Close is not part of this
// interface; type-assert io.Closer when the backend owns a handle.
type Backend interface {
	Log(ctx context.Context, level Level, msg string, attrs ...Attr)
	Sync() error
	With(options ...Option) Backend
}

// slogBackend is an optional capability for backends that can expose a
// *slog.Logger, allowing WithLoggerBackend to also route the standard library's
// slog output through the configured backend.
type SlogBackend interface {
	SlogLogger(options ...Option) *slog.Logger
}
