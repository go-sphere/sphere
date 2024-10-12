package log

import (
	"github.com/tbxark/go-base-api/pkg/log/logfields"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"sync"
)

type Fields map[string]interface{}

// Logger is a contract for the logger
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Debugw(message string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Infow(message string, args ...interface{})

	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Warnw(message string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Errorw(message string, args ...interface{})

	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	Panicw(message string, args ...interface{})

	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Fatalw(message string, args ...interface{})

	WithFields(keyValues Fields) Logger
}

type zapLogger struct {
	sugarLogger *zap.SugaredLogger
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

func (l *zapLogger) WithFields(fields Fields) Logger {
	var f = make([]interface{}, 0)
	for k, v := range fields {
		f = append(f, k)
		f = append(f, v)
	}
	logger := l.sugarLogger.With(f...)
	return &zapLogger{logger}
}

var _ Logger = &zapLogger{}

var (
	mu  sync.Mutex
	std = newLogger(NewOptions())
)

func Init(opts *Options, fields ...logfields.Field) {
	mu.Lock()
	defer mu.Unlock()
	std = newLogger(opts, fields...)
}

func Sync() error {
	return std.sugarLogger.Sync()
}

func NewLogger(opts *Options) Logger {
	return newLogger(opts)
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

func With(fields Fields) Logger {
	if len(fields) == 0 {
		return std
	}
	return std.WithFields(fields)
}

func WithEx(info ...string) Logger {
	fields := Fields{}
	for i := 0; i < len(info); i += 2 {
		fields[info[i]] = info[i+1]
	}
	return std.WithFields(fields)
}

func DisableCaller() Logger {
	return &zapLogger{std.sugarLogger.WithOptions(zap.WithCaller(false))}
}

func Debug(args ...interface{}) {
	std.Debug(args...)
}

func Info(args ...interface{}) {
	std.Info(args...)
}

func Warn(args ...interface{}) {
	std.Warn(args...)
}

func Error(args ...interface{}) {
	std.Error(args...)
}

func Panic(args ...interface{}) {
	std.Panic(args...)
}

func Fatal(args ...interface{}) {
	std.Fatal(args...)
}

func Debugf(format string, args ...interface{}) {
	std.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	std.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	std.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	std.Errorf(format, args...)
}

func Panicf(format string, args ...interface{}) {
	std.Panicf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	std.Fatalf(format, args...)
}

func Debugw(message string, args ...interface{}) {
	std.Debugw(message, args...)
}

func Infow(message string, args ...interface{}) {
	std.Infow(message, args...)
}

func Warnw(message string, args ...interface{}) {
	std.Warnw(message, args...)
}

func Errorw(message string, args ...interface{}) {
	std.Errorw(message, args...)
}

func Panicw(message string, args ...interface{}) {
	std.Panicw(message, args...)
}

func Fatalw(message string, args ...interface{}) {
	std.Fatalw(message, args...)
}

func ZapLogger() *zap.Logger {
	return std.sugarLogger.Desugar().WithOptions(zap.WithCaller(false))
}
