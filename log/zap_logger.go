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
	core := newZapCore(config)
	return &zapLogger{
		logger: zap.New(core).
			With(mapToZapFields(opts.attrs)...).
			WithOptions(zapOptions(opts)...).
			Sugar(),
	}
}

func newZapCore(config *Config) zapcore.Core {
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

	return zapcore.NewTee(nodes...)
}

func (z *zapLogger) Debug(msg string, args ...any) {
	z.logger.Debugw(msg, args...)
}

func (z *zapLogger) Info(msg string, args ...any) {
	z.logger.Infow(msg, args...)
}

func (z *zapLogger) Warn(msg string, args ...any) {
	z.logger.Warnw(msg, args...)
}

func (z *zapLogger) Error(msg string, args ...any) {
	z.logger.Errorw(msg, args...)
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

func (z *zapLogger) With(options ...Option) *zapLogger {
	opts := newOptions(options...)
	return &zapLogger{
		logger: z.logger.
			Desugar().
			With(mapToZapFields(opts.attrs)...).
			WithOptions(zapOptions(opts)...).
			Sugar(),
	}
}

func zapOptions(o *options) []zap.Option {
	opts := make([]zap.Option, 0, 3)
	switch o.addCaller {
	case AddCallerStatusEnable:
		opts = append(opts, zap.WithCaller(true))
	case AddCallerStatusDisable:
		opts = append(opts, zap.WithCaller(false))
	default:
		break
	}
	if o.addStackAt != zapcore.InvalidLevel {
		opts = append(opts, zap.AddStacktrace(o.addStackAt))
	}
	if o.callerSkip != 0 {
		opts = append(opts, zap.AddCallerSkip(o.callerSkip))
	}
	return opts
}

func mapToZapFields(attrs map[string]any) []zap.Field {
	fields := make([]zap.Field, 0, len(attrs))
	for k, v := range attrs {
		fields = append(fields, zap.Any(k, v))
	}
	return fields
}
