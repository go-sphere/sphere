package zapx

import (
	"context"
	"io"
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
	// file is the rotating log writer opened by NewBackend, or nil when this
	// backend was derived through With. Only the backend that opened the writer
	// closes it, so a derived logger can never release its parent's handle.
	file io.Closer
}

// coreCallerOffset compensates for:
// 1) Backend.Log itself
// 2) core logger call sites (both package-level log.* and logger instance methods).
const coreCallerOffset = 2

// NewBackend creates a zap-based backend.
func NewBackend(conf Config, options ...corelog.Option) *Backend {
	resolved := corelog.NewOptions(options...)
	core, file := newCore(conf)
	logger := zap.New(core).Named(resolved.Name).WithOptions(zapOptions(resolved)...)
	if len(resolved.Attrs) > 0 {
		logger = logger.With(MapToZapFields(resolved.Attrs)...)
	}
	backend := newBackendWithLogger(logger)
	backend.file = file
	return backend
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

// Close releases the rotating log file opened by NewBackend. It is a no-op for
// backends derived through With, which share the writer but do not own it, and
// for backends configured without Config.File.FileName.
//
// Close does not flush: call Sync first if buffered entries must reach disk.
// Logging through a closed backend is not an error — lumberjack reopens the file
// on the next write — so Close is about releasing the handle, not sealing the
// backend.
func (z *Backend) Close() error {
	if z.file == nil {
		return nil
	}
	return z.file.Close()
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

// newCore builds the zap core for conf and returns the rotating file writer it
// opened, or nil when conf declares no file sink. The caller owns the writer.
func newCore(conf Config) (zapcore.Core, io.Closer) {
	if conf.Level == "" {
		conf.Level = defaultLevel
	}
	levelRaw, err := zapcore.ParseLevel(conf.Level)
	if err != nil {
		levelRaw = zap.InfoLevel
	}
	level := zap.NewAtomicLevelAt(levelRaw)

	var nodes []zapcore.Core
	var file io.Closer

	if !conf.Console.Disable {
		developmentCfg := zap.NewDevelopmentEncoderConfig()
		developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)
		pc := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
		nodes = append(nodes, pc)
	}

	if conf.File.FileName != "" {
		productionCfg := zap.NewProductionEncoderConfig()
		productionCfg.TimeKey = "timestamp"
		productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		fileEncoder := zapcore.NewJSONEncoder(productionCfg)
		rotator := &lumberjack.Logger{
			Filename:   conf.File.FileName,
			MaxSize:    conf.File.MaxSize,
			MaxBackups: conf.File.MaxBackups,
			MaxAge:     conf.File.MaxAge,
		}
		file = rotator
		pc := zapcore.NewCore(fileEncoder, zapcore.AddSync(rotator), level)
		nodes = append(nodes, pc)
	}

	if len(nodes) == 0 {
		return zapcore.NewNopCore(), file
	}
	return zapcore.NewTee(nodes...), file
}
