package log

import "context"

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
