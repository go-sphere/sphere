package log

import (
	"log/slog"
	"sort"

	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

// NewSlogLogger creates a new structured logger using the standard library's slog interface.
// This provides compatibility with Go's standard structured logging while using zap as the backend.
func NewSlogLogger(config *Config, options ...Option) *slog.Logger {
	if config == nil {
		config = NewDefaultConfig()
	}
	core := newZapCore(config)
	return newSlogLogger(
		core,
		options...,
	)
}

func newSlogLogger(core zapcore.Core, options ...Option) *slog.Logger {
	opts := newOptions(options...)
	return slog.New(
		zapslog.NewHandler(core, zapSlogOptions(opts)...).
			WithAttrs(mapToSlogAttrs(opts.attrs)),
	)
}

func zapSlogOptions(o *options) []zapslog.HandlerOption {
	opts := make([]zapslog.HandlerOption, 0, 4)
	switch o.addCaller {
	case AddCallerStatusEnable:
		opts = append(opts, zapslog.WithCaller(true))
	case AddCallerStatusDisable:
		opts = append(opts, zapslog.WithCaller(false))
	default:
		break
	}
	if o.name != "" {
		opts = append(opts, zapslog.WithName(o.name))
	}
	if o.addStackAt != zapcore.InvalidLevel {
		opts = append(opts, zapslog.AddStacktraceAt(zapLevelToSlogLevel(o.addStackAt)))
	}
	if o.callerSkip != 0 {
		opts = append(opts, zapslog.WithCallerSkip(o.callerSkip))
	}
	return opts
}

func mapToSlogAttrs(attrs map[string]any) []slog.Attr {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	attrsList := make([]slog.Attr, 0, len(attrs))
	for _, k := range keys {
		attrsList = append(attrsList, slog.Any(k, attrs[k]))
	}
	return attrsList
}

func zapLevelToSlogLevel(l zapcore.Level) slog.Level {
	switch {
	case l > zapcore.ErrorLevel:
		// Keep levels above error distinct so stacktrace thresholds remain precise.
		return slog.LevelError + slog.Level(l-zapcore.ErrorLevel)
	case l == zapcore.ErrorLevel:
		return slog.LevelError
	case l == zapcore.WarnLevel:
		return slog.LevelWarn
	case l == zapcore.InfoLevel:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}
