package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type zapLogger struct {
	logger *zap.SugaredLogger
}

func newZapLogger(config *Config, options ...Option) *zapLogger {
	opts := newOptions(options...)
	levelRaw, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		levelRaw = zap.InfoLevel
	}
	level := zap.NewAtomicLevelAt(levelRaw)

	var nodes []zapcore.Core

	if config.Console == nil || !config.Console.Disable {
		developmentCfg := zap.NewDevelopmentEncoderConfig()
		developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)
		pc := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
		nodes = append(nodes, pc)
	}

	if config.File != nil {
		productionCfg := zap.NewProductionEncoderConfig()
		productionCfg.TimeKey = "timestamp"
		productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		fileEncoder := zapcore.NewJSONEncoder(productionCfg)
		file := zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.File.FileName,
			MaxSize:    config.File.MaxSize, // megabytes
			MaxBackups: config.File.MaxBackups,
			MaxAge:     config.File.MaxAge, // days
		})
		pc := zapcore.NewCore(fileEncoder, file, level)
		nodes = append(nodes, pc)
	}

	core := zapcore.NewTee(
		nodes...,
	)

	return &zapLogger{
		logger: zap.New(core,
			zap.WithCaller(opts.addCaller),
			zap.AddCallerSkip(opts.callerSkip),
			zap.AddStacktrace(opts.addStackAt),
		).With(attrsToZapFields(opts.attrs)...).Sugar(),
	}
}

func attrsToZapFields(attrs map[string]any) []zap.Field {
	fields := make([]zap.Field, 0, len(attrs))
	for k, v := range attrs {
		fields = append(fields, zap.Any(k, v))
	}
	return fields
}

func (z *zapLogger) Debug(msg string, attrs ...any) {
	z.logger.Debugw(msg, attrs...)
}

func (z *zapLogger) Info(msg string, attrs ...any) {
	z.logger.Infow(msg, attrs...)
}

func (z *zapLogger) Warn(msg string, attrs ...any) {
	z.logger.Warnw(msg, attrs...)
}

func (z *zapLogger) Error(msg string, attrs ...any) {
	z.logger.Errorw(msg, attrs...)
}

func (z *zapLogger) Debugf(format string, args ...any) {
	z.logger.Debugf(format, args...)
}

func (z *zapLogger) Infof(format string, args ...any) {
	z.logger.Infof(format, args...)
}

func (z *zapLogger) Warnf(format string, args ...any) {
	z.logger.Warnf(format, args...)
}

func (z *zapLogger) Errorf(format string, args ...any) {
	z.logger.Errorf(format, args...)
}
