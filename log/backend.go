package log

import "context"

// Level is a backend-agnostic log level.
type Level int8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Backend is the pluggable logging backend interface.
type Backend interface {
	Log(ctx context.Context, level Level, msg string, attrs ...Attr)
	Sync() error
	With(options ...Option) Backend
}
