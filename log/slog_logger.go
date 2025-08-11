package log

import (
	"log/slog"

	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

func NewSlogger(config *Config, options ...Option) *slog.Logger {
	logger := newZapLogger(config, options...)
	return newSlogLogger(logger.logger.Desugar().Core(), options...)
}

func newSlogLogger(core zapcore.Core, options ...Option) *slog.Logger {
	opts := newOptions(options...)
	return slog.New(zapslog.NewHandler(
		core,
		zapslog.WithCaller(opts.addCaller),
		zapslog.WithCallerSkip(opts.callerSkip),
		zapslog.AddStacktraceAt(convertZapLevel(opts.addStackAt)),
	).WithAttrs(attrsToSlogAttrs(opts.attrs)))
}

func attrsToSlogAttrs(attrs map[string]any) []slog.Attr {
	attrsList := make([]slog.Attr, 0, len(attrs))
	for k, v := range attrs {
		attrsList = append(attrsList, slog.Any(k, v))
	}
	return attrsList
}

func convertZapLevel(l zapcore.Level) slog.Level {
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
