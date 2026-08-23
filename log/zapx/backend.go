// Package zapx is a log.Backend on uber-go/zap: colored console on stdout
// and optional lumberjack JSON file rotation.
//
// NewBackend owns the file handle; derived With loggers do not. Close
// releases the file; lumberjack reopens on the next write, so Close is not
// "seal the backend". Sync before Close if buffered entries must land.
// log.WithMinLevel is ignored; set Config.Level (zap level string, default
// "info"). Invalid Level falls back to Info. FileConfig.MaxSize is MB,
// MaxAge is days, MaxBackups is count.
package zapx

import (
	"context"
	"fmt"
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
	// name accumulates the logger names applied through NewBackend and With.
	// zap keeps the name on the *zap.Logger rather than in its Core, and the
	// slog bridge is built from the Core alone, so without tracking it here
	// every record routed through SlogHandler would lose its origin. Persistent
	// attrs do not need this: With pushes them down into the Core.
	name string
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
	backend := newBackendWithLogger(logger, resolved.Name)
	backend.file = file
	return backend
}

func newBackendWithLogger(zapLogger *zap.Logger, name string) *Backend {
	return &Backend{
		zapLogger:  zapLogger,
		coreLogger: zapLogger.WithOptions(zap.AddCallerSkip(coreCallerOffset)),
		name:       name,
	}
}

// joinName mirrors zap's own naming, which joins segments with a dot.
func joinName(base, next string) string {
	switch {
	case next == "":
		return base
	case base == "":
		return next
	default:
		return base + "." + next
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

// Log writes one entry, degrading to a diagnostic when an attribute panics.
//
// The recover has to wrap the write, not just field construction: zap.Any is
// lazy, so a value's MarshalJSON or Stringer runs when the entry is encoded.
// Attribute values are arbitrary caller data, and without this a panic in one of
// them propagated out of Log and aborted the goroutine that logged — on a value
// the stdio backend merely degraded on, so swapping backends changed whether an
// application survived its own logging. Encoding fills a buffer before writing,
// so a panic partway through leaves nothing on the output and the replacement
// entry is the only one emitted.
func (z *Backend) Log(ctx context.Context, level corelog.Level, msg string, attrs ...corelog.Attr) {
	logger := z.logEntryLogger()
	defer func() {
		if r := recover(); r != nil {
			z.write(logger, level, msg, []zap.Field{zap.String("attr_error", fmt.Sprint(r))})
		}
	}()

	fields := make([]zap.Field, 0, len(attrs))
	for _, a := range attrs {
		fields = append(fields, AttrToZapField(a))
	}
	z.write(logger, level, msg, fields)
}

func (z *Backend) write(logger *zap.Logger, level corelog.Level, msg string, fields []zap.Field) {
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

// With returns a derived backend that shares the parent's sinks. The derived
// backend does not own the file handle, so Close on it is a no-op.
// log.WithMinLevel is ignored; the zap level is fixed by Config.Level.
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
	return newBackendWithLogger(logger, joinName(z.name, resolved.Name))
}

// Sync flushes buffered zap entries. Call Sync before Close if file entries
// must reach disk. Console (stdout) sync errors are discarded.
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

// SlogHandler returns a slog.Handler on this backend's zap core. The logger
// name is applied here because the core does not store it.
func (z *Backend) SlogHandler(options ...corelog.Option) slog.Handler {
	resolved := corelog.NewOptions(options...)
	// Carry this backend's own name into the bridge. The handler is built from
	// the Core, which does not hold the name, so records would otherwise arrive
	// unattributed — including on the boot.WithLoggerBackend path, which calls
	// SlogLogger with no options at all.
	resolved.Name = joinName(z.name, resolved.Name)
	var h slog.Handler = zapslog.NewHandler(z.zapLogger.Core(), zapSlogOptions(resolved)...)
	if len(resolved.Attrs) > 0 {
		h = h.WithAttrs(mapToSlogAttrs(resolved.Attrs))
	}
	return h
}

// SlogLogger returns a *slog.Logger whose handler is SlogHandler.
func (z *Backend) SlogLogger(options ...corelog.Option) *slog.Logger {
	return slog.New(z.SlogHandler(options...))
}

// ZapLogger returns the raw *zap.Logger without the extra caller skip used
// by Backend.Log.
func (z *Backend) ZapLogger() *zap.Logger {
	return z.zapLogger
}

// consoleSyncer drops the error from syncing the console sink.
//
// Sync on os.Stdout issues a real fsync, which the kernel rejects for a pipe or
// character device with EINVAL/ENOTTY/EBADF. Under a container, systemd or CI,
// stdout is essentially always a pipe, so the error is constant and carries no
// information: there is nothing buffered on this side to flush. Propagating it
// made Sync unusable as a success signal, because any configuration including a
// console sink failed every time — masking a genuine flush failure from a file
// sink, which errors.Join reports through the same value.
type consoleSyncer struct {
	zapcore.WriteSyncer
}

func (c consoleSyncer) Sync() error {
	_ = c.WriteSyncer.Sync()
	return nil
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
		pc := zapcore.NewCore(consoleEncoder, consoleSyncer{zapcore.AddSync(os.Stdout)}, level)
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
