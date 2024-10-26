package log

import (
	"github.com/tbxark/sphere/pkg/log/logfields"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

var _ Logger = &zapLogger{}

type zapLogger struct {
	sugarLogger *zap.SugaredLogger
}

func newLogger(opts *Options, fields ...logfields.Field) *zapLogger {

	levelRaw, err := zapcore.ParseLevel(opts.Level)
	if err != nil {
		levelRaw = zap.InfoLevel
	}
	level := zap.NewAtomicLevelAt(levelRaw)

	developmentCfg := zap.NewDevelopmentEncoderConfig()
	developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)
	var nodes []zapcore.Core

	if opts.ConsoleOutAsync {
		nodes = append(nodes, zapcore.NewCore(consoleEncoder, os.Stdout, level))
	} else {
		stdout := zapcore.AddSync(os.Stdout)
		nodes = append(nodes, zapcore.NewCore(consoleEncoder, stdout, level))
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

		pc := zapcore.NewCore(fileEncoder, file, level).With(fields)
		nodes = append(nodes, pc)
	}

	core := zapcore.NewTee(
		nodes...,
	)
	z := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(2),
		zap.AddStacktrace(zap.ErrorLevel),
	)
	return &zapLogger{
		sugarLogger: z.Sugar(),
	}
}

func (l *zapLogger) Debug(args ...interface{}) {
	l.sugarLogger.Debug(args...)
}

func (l *zapLogger) Info(args ...interface{}) {
	l.sugarLogger.Info(args...)
}

func (l *zapLogger) Warn(args ...interface{}) {
	l.sugarLogger.Warn(args...)
}

func (l *zapLogger) Error(args ...interface{}) {
	l.sugarLogger.Error(args...)
}

func (l *zapLogger) Panic(args ...interface{}) {
	l.sugarLogger.Panic(args...)
}

func (l *zapLogger) Fatal(args ...interface{}) {
	l.sugarLogger.Fatal(args...)
}

func (l *zapLogger) Debugf(format string, args ...interface{}) {
	l.sugarLogger.Debugf(format, args...)
}

func (l *zapLogger) Infof(format string, args ...interface{}) {
	l.sugarLogger.Infof(format, args...)
}

func (l *zapLogger) Warnf(format string, args ...interface{}) {
	l.sugarLogger.Warnf(format, args...)
}

func (l *zapLogger) Errorf(format string, args ...interface{}) {
	l.sugarLogger.Errorf(format, args...)
}

func (l *zapLogger) Fatalf(format string, args ...interface{}) {
	l.sugarLogger.Fatalf(format, args...)
}

func (l *zapLogger) Panicf(format string, args ...interface{}) {
	l.sugarLogger.Panicf(format, args...)
}

func (l *zapLogger) Debugw(message string, args ...interface{}) {
	l.sugarLogger.Debugw(message, args...)
}

func (l *zapLogger) Infow(message string, args ...interface{}) {
	l.sugarLogger.Infow(message, args...)
}

func (l *zapLogger) Warnw(message string, args ...interface{}) {
	l.sugarLogger.Warnw(message, args...)
}

func (l *zapLogger) Errorw(message string, args ...interface{}) {
	l.sugarLogger.Errorw(message, args...)
}

func (l *zapLogger) Panicw(message string, args ...interface{}) {
	l.sugarLogger.Panicw(message, args...)
}

func (l *zapLogger) Fatalw(message string, args ...interface{}) {
	l.sugarLogger.Fatalw(message, args...)
}

func (l *zapLogger) WithFields(fields map[string]interface{}) Logger {
	var f = make([]zap.Field, 0)
	for k, v := range fields {
		f = append(f, zap.Any(k, v))
	}
	logger := l.sugarLogger.With(f)
	return &zapLogger{logger}
}
