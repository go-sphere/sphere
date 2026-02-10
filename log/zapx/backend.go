package zapx

import (
	"context"
	"log/slog"
	"os"

	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Backend is the zap implementation of corelog.Backend.
type Backend struct {
	// zapLogger is the raw zap logger exposed to callers via ZapLogger().
	// It does not include backend-specific caller skip adjustments.
	zapLogger *zap.Logger
	// coreLogger is used by Backend.Log and pre-applies caller skip so core log APIs
	// report the user's call site instead of wrapper frames.
	coreLogger *zap.Logger
}

// coreCallerOffset compensates for:
// 1) Backend.Log itself
// 2) core logger call sites (both package-level log.* and logger instance methods).
const coreCallerOffset = 2

// NewBackend creates a zap-based backend.
func NewBackend(config *Config, options ...corelog.Option) *Backend {
	resolved := corelog.NewOptions(options...)
	core := newCore(config)
	logger := zap.New(core).Named(resolved.Name).WithOptions(zapOptions(resolved)...)
	if len(resolved.Attrs) > 0 {
		logger = logger.With(MapToZapFields(resolved.Attrs)...)
	}
	return newBackendWithLogger(logger)
}

func newBackendWithLogger(zapLogger *zap.Logger) *Backend {
	return &Backend{
		zapLogger:  zapLogger,
		coreLogger: zapLogger.WithOptions(zap.AddCallerSkip(coreCallerOffset)),
	}
}

func (z *Backend) logEntryLogger() *zap.Logger {
	if z.coreLogger != nil {
		return z.coreLogger
	}
	if z.zapLogger != nil {
		return z.zapLogger.WithOptions(zap.AddCallerSkip(coreCallerOffset))
	}
	return zap.NewNop()
}

func (z *Backend) Log(ctx context.Context, level corelog.Level, msg string, attrs ...corelog.Attr) {
	logger := z.logEntryLogger()
	fields := make([]zap.Field, 0, len(attrs))
	for _, a := range attrs {
		fields = append(fields, AttrToZapField(a))
	}

	switch level {
	case corelog.LevelDebug:
		logger.Debug(msg, fields...)
	case corelog.LevelInfo:
		logger.Info(msg, fields...)
	case corelog.LevelWarn:
		logger.Warn(msg, fields...)
	case corelog.LevelError:
		logger.Error(msg, fields...)
	default:
		logger.Info(msg, fields...)
	}
}

func (z *Backend) With(options ...corelog.Option) corelog.Backend {
	resolved := corelog.NewOptions(options...)
	logger := z.zapLogger
	if resolved.Name != "" {
		logger = logger.Named(resolved.Name)
	}
	logger = logger.WithOptions(zapOptions(resolved)...)
	if len(resolved.Attrs) > 0 {
		logger = logger.With(MapToZapFields(resolved.Attrs)...)
	}
	return newBackendWithLogger(logger)
}

func (z *Backend) Sync() error {
	return z.zapLogger.Sync()
}

func (z *Backend) SlogHandler(options ...corelog.Option) slog.Handler {
	resolved := corelog.NewOptions(options...)
	var h slog.Handler = zapslog.NewHandler(z.zapLogger.Core(), zapSlogOptions(resolved)...)
	if len(resolved.Attrs) > 0 {
		h = h.WithAttrs(mapToSlogAttrs(resolved.Attrs))
	}
	return h
}

func (z *Backend) SlogLogger(options ...corelog.Option) *slog.Logger {
	return slog.New(z.SlogHandler(options...))
}

func (z *Backend) ZapLogger() *zap.Logger {
	return z.zapLogger
}

func newCore(config *Config) zapcore.Core {
	if config == nil {
		config = NewDefaultConfig()
	}
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
			MaxSize:    config.File.MaxSize,
			MaxBackups: config.File.MaxBackups,
			MaxAge:     config.File.MaxAge,
		})
		pc := zapcore.NewCore(fileEncoder, file, level)
		nodes = append(nodes, pc)
	}

	if len(nodes) == 0 {
		return zapcore.NewNopCore()
	}
	return zapcore.NewTee(nodes...)
}
