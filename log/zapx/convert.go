package zapx

import (
	"fmt"
	"log/slog"
	"sort"

	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func MapToZapFields(attrs map[string]any) []zap.Field {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fields := make([]zap.Field, 0, len(attrs))
	for _, k := range keys {
		v := attrs[k]
		fields = append(fields, zap.Any(k, v))
	}
	return fields
}

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
		return zap.Any(attr.Key, groupToMap(v.Group()))
	case slog.KindAny:
		if err, ok := v.Any().(error); ok && attr.Key == "error" {
			return zap.Error(err)
		}
		return zap.Any(attr.Key, v.Any())
	default:
		return zap.Any(attr.Key, fmt.Sprint(v.Any()))
	}
}

func mapToSlogAttrs(attrs map[string]any) []slog.Attr {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]slog.Attr, 0, len(attrs))
	for _, k := range keys {
		out = append(out, slog.Any(k, attrs[k]))
	}
	return out
}

func groupToMap(attrs []slog.Attr) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[a.Key] = slogValueToAny(a.Value)
	}
	return m
}

func slogValueToAny(v slog.Value) any {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration()
	case slog.KindTime:
		return v.Time()
	case slog.KindGroup:
		return groupToMap(v.Group())
	case slog.KindAny:
		return v.Any()
	default:
		return v.Any()
	}
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
