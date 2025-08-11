package log

import (
	"log/slog"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type zapLogger struct {
	core  zapcore.Core
	attrs []slog.Attr
}

func newZapLogHandler(opts *Options, attrs []slog.Attr) *zapLogger {
	levelRaw, err := zapcore.ParseLevel(opts.Level)
	if err != nil {
		levelRaw = zap.InfoLevel
	}
	level := zap.NewAtomicLevelAt(levelRaw)

	var nodes []zapcore.Core

	if opts.Console == nil || !opts.Console.Disable {
		developmentCfg := zap.NewDevelopmentEncoderConfig()
		developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)
		pc := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
		nodes = append(nodes, pc)
	}

	if opts.File != nil {
		productionCfg := zap.NewProductionEncoderConfig()
		productionCfg.TimeKey = "timestamp"
		productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		fileEncoder := zapcore.NewJSONEncoder(productionCfg)
		file := zapcore.AddSync(&lumberjack.Logger{
			Filename:   opts.File.FileName,
			MaxSize:    opts.File.MaxSize, // megabytes
			MaxBackups: opts.File.MaxBackups,
			MaxAge:     opts.File.MaxAge, // days
		})
		pc := zapcore.NewCore(fileEncoder, file, level)
		nodes = append(nodes, pc)
	}

	core := zapcore.NewTee(
		nodes...,
	)
	return &zapLogger{
		core:  core,
		attrs: attrs,
	}
}

func (z *zapLogger) Handler(options ...zapslog.HandlerOption) slog.Handler {
	return zapslog.NewHandler(z.core, options...).WithAttrs(z.attrs)
}
