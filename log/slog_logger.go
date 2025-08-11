package log

import (
	"log/slog"

	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

func NewSlogLogger(config *Config, options ...Option) *slog.Logger {
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
	opts := make([]zapslog.HandlerOption, 0, 3)
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
	attrsList := make([]slog.Attr, 0, len(attrs))
	for k, v := range attrs {
		attrsList = append(attrsList, slog.Any(k, v))
	}
	return attrsList
}

func zapLevelToSlogLevel(l zapcore.Level) slog.Level {
	switch {
	case l >= zapcore.ErrorLevel:
		return slog.LevelError
	case l >= zapcore.WarnLevel:
		return slog.LevelWarn
	case l >= zapcore.InfoLevel:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}
