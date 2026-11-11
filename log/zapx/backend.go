package zapx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	corelog "github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/internal/callsite"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Backend is the zap implementation of log.Backend; it also implements
// log.SlogBackend and io.Closer. Create it with NewBackend; the zero value is
// not usable. A Backend is safe for concurrent use. A panic while encoding an
// attribute value is recovered and logged as an attr_error field instead.
//
// With log.AddCaller, entries written through Log carry the first stack frame
// outside the log package tree as their caller, so the user's call site is
// reported through MultiBackend and WrapBackendWithContextMerge as well.
type Backend struct {
	// zapLogger is the raw zap logger exposed to callers via ZapLogger(). zap
	// computes its caller itself, so it is correct for direct zap calls.
	zapLogger *zap.Logger
	// coreLogger is used by Backend.Log. zap's own caller lookup is disabled on
	// it, because Log sets the entry caller itself; the caller skip only trims
	// Backend.Log and the facade method from stack traces.
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
	// addCaller and stackAt are the resolved caller and stack options, kept
	// because zap does not expose them and SlogHandler inherits them.
	addCaller bool
	stackAt   *corelog.Level
}

// coreCallerOffset is the number of frames between zap's Check and the user
// on the direct path: Backend.Log and the facade method.
const coreCallerOffset = 2

// NewBackend creates a zap-based backend from conf and options. It never
// fails: an invalid conf.Level falls back to info, and a config with the
// console disabled and no FileName discards every entry. The log file, when
// configured, is opened lazily on first write. The caller owns the returned
// Backend and should Sync then Close it at shutdown. log.WithMinLevel is
// ignored.
func NewBackend(conf Config, options ...corelog.Option) *Backend {
	core, file := newCore(conf)
	backend := newBackendFromCore(core, options...)
	backend.file = file
	return backend
}

// newBackendFromCore builds a Backend over core with options resolved as in
// NewBackend. The returned backend owns no file.
func newBackendFromCore(core zapcore.Core, options ...corelog.Option) *Backend {
	root := &Backend{zapLogger: zap.New(core)}
	return root.with(options...)
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

// Log writes one entry, degrading to a diagnostic when an attribute panics.
//
// The recover has to wrap the write, not just field construction: zap.Any is
// lazy, so a value's MarshalJSON or Stringer runs when the entry is encoded.
// Encoding fills a buffer before writing, so a panic partway through leaves
// nothing on the output and the replacement entry is the only one emitted.
//
// A level outside LevelDebug..LevelError is clamped to the nearest of them.
// Attrs are not resolved when the level is disabled, so an expensive LogValuer
// costs nothing then.
func (z *Backend) Log(ctx context.Context, level corelog.Level, msg string, attrs ...corelog.Attr) {
	zapLevel := logLevelToZapLevel(level)
	ce := z.coreLogger.Check(zapLevel, msg)
	if ce == nil {
		return
	}
	var caller zapcore.EntryCaller
	if z.addCaller {
		if frame, ok := callsite.Frame(); ok {
			caller = zapcore.EntryCaller{
				Defined:  true,
				PC:       frame.PC,
				File:     frame.File,
				Line:     frame.Line,
				Function: frame.Function,
			}
		}
		ce.Caller = caller
	}
	defer func() {
		if r := recover(); r != nil {
			if fallback := z.coreLogger.Check(zapLevel, msg); fallback != nil {
				if z.addCaller {
					fallback.Caller = caller
				}
				fallback.Write(zap.String("attr_error", fmt.Sprint(r)))
			}
		}
	}()

	fields := make([]zap.Field, 0, len(attrs))
	for _, a := range attrs {
		fields = append(fields, AttrToZapField(a))
	}
	ce.Write(fields...)
}

// With returns a derived backend that shares the parent's sinks. The derived
// backend does not own the file handle, so Close on it is a no-op.
// log.WithMinLevel is ignored; the zap level is fixed by Config.Level.
func (z *Backend) With(options ...corelog.Option) corelog.Backend {
	return z.with(options...)
}

func (z *Backend) with(options ...corelog.Option) *Backend {
	resolved := corelog.NewOptions(options...)
	logger := z.zapLogger
	if resolved.Name != "" {
		logger = logger.Named(resolved.Name)
	}
	logger = logger.WithOptions(zapOptions(resolved)...)
	if len(resolved.Attrs) > 0 {
		logger = logger.With(MapToZapFields(resolved.Attrs)...)
	}
	next := &Backend{
		zapLogger:  logger,
		coreLogger: logger.WithOptions(zap.WithCaller(false), zap.AddCallerSkip(coreCallerOffset)),
		name:       joinName(z.name, resolved.Name),
		addCaller:  z.addCaller,
		stackAt:    z.stackAt,
	}
	switch resolved.AddCaller {
	case corelog.AddCallerStatusEnable:
		next.addCaller = true
	case corelog.AddCallerStatusDisable:
		next.addCaller = false
	default:
	}
	if resolved.AddStackAt != nil {
		next.stackAt = resolved.AddStackAt
	}
	return next
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

// SlogHandler returns a slog.Handler on this backend's zap core. It inherits
// the backend's name, caller and stack settings; options are applied on top.
// The caller is the slog call site.
func (z *Backend) SlogHandler(options ...corelog.Option) slog.Handler {
	resolved := corelog.NewOptions(options...)
	// The handler is built from the Core, which holds none of these settings,
	// so they are carried over here — including on the boot.WithLoggerBackend
	// path, which calls SlogLogger with no options at all.
	resolved.Name = joinName(z.name, resolved.Name)
	if resolved.AddCaller == corelog.AddCallerStatusKeep && z.addCaller {
		resolved.AddCaller = corelog.AddCallerStatusEnable
	}
	if resolved.AddStackAt == nil {
		resolved.AddStackAt = z.stackAt
	}
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
// by Backend.Log. Options derive a logger without changing this backend.
func (z *Backend) ZapLogger(options ...corelog.Option) *zap.Logger {
	if len(options) == 0 {
		return z.zapLogger
	}
	return z.with(options...).zapLogger
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
