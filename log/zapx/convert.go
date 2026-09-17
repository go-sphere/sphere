package zapx

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"

	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// MapToZapFields converts attrs into zap fields in sorted key order.
func MapToZapFields(attrs map[string]any) []zap.Field {
	keys := slices.Sorted(maps.Keys(attrs))

	fields := make([]zap.Field, 0, len(attrs))
	for _, k := range keys {
		v := attrs[k]
		fields = append(fields, zap.Any(k, v))
	}
	return fields
}

// AttrToZapField converts attr to a zap field. An error stored under "error"
// uses zap.Error even if it also implements a zap marshaling interface. Other
// arbitrary values follow zap.Any's dispatch rules, including inside groups.
func AttrToZapField(attr corelog.Attr) zap.Field {
	v := attr.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return zap.String(attr.Key, v.String())
	case slog.KindInt64:
		return zap.Int64(attr.Key, v.Int64())
	case slog.KindUint64:
		return zap.Uint64(attr.Key, v.Uint64())
	case slog.KindFloat64:
		return zap.Float64(attr.Key, v.Float64())
	case slog.KindBool:
		return zap.Bool(attr.Key, v.Bool())
	case slog.KindDuration:
		return zap.Duration(attr.Key, v.Duration())
	case slog.KindTime:
		return zap.Time(attr.Key, v.Time())
	case slog.KindGroup:
		// Encode the group through an ObjectMarshaler rather than flattening it
		// into a map[string]any: a map goes through zap's reflection/JSON path,
		// which renders a nested time.Duration as its nanosecond integer, loses
		// error fields and collapses duplicate keys, making the output depend on
		// nesting depth.
		return zap.Object(attr.Key, groupObject(v.Group()))
	case slog.KindAny:
		if err, ok := v.Any().(error); ok && attr.Key == "error" {
			return zap.Error(err)
		}
		return zap.Any(attr.Key, v.Any())
	default:
		return zap.Any(attr.Key, fmt.Sprint(v.Any()))
	}
}

// groupObject encodes a slog group with the same typed encoders used for
// top-level attrs, recursively.
type groupObject []slog.Attr

func (g groupObject) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for _, a := range g {
		AttrToZapField(a).AddTo(enc)
	}
	return nil
}

func mapToSlogAttrs(attrs map[string]any) []slog.Attr {
	keys := slices.Sorted(maps.Keys(attrs))

	out := make([]slog.Attr, 0, len(attrs))
	for _, k := range keys {
		out = append(out, slog.Any(k, attrs[k]))
	}
	return out
}

func logLevelToZapLevel(level corelog.Level) zapcore.Level {
	switch level {
	case corelog.LevelDebug:
		return zapcore.DebugLevel
	case corelog.LevelInfo:
		return zapcore.InfoLevel
	case corelog.LevelWarn:
		return zapcore.WarnLevel
	case corelog.LevelError:
		return zapcore.ErrorLevel
	default:
		return zapcore.ErrorLevel
	}
}

func logLevelToSlogLevel(level corelog.Level) slog.Level {
	switch level {
	case corelog.LevelDebug:
		return slog.LevelDebug
	case corelog.LevelInfo:
		return slog.LevelInfo
	case corelog.LevelWarn:
		return slog.LevelWarn
	case corelog.LevelError:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}
