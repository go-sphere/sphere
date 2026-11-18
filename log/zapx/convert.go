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
		if sv, ok := v.(slog.Value); ok {
			if f := AttrToZapField(corelog.Attr{Key: k, Value: sv}); f.Type != zapcore.SkipType {
				fields = append(fields, f)
			}
			continue
		}
		if k == "" {
			continue
		}
		fields = append(fields, zap.Any(k, v))
	}
	return fields
}

const (
	maxGroupDepth = 32
	maxGroupNodes = 4096
)

// AttrToZapField converts attr to a zap field. An error stored under "error"
// uses zap.Error even if it also implements a zap marshaling interface. Other
// arbitrary values follow zap.Any's dispatch rules, including inside groups.
// Empty-key non-group attrs and empty groups return zap.Skip(); empty-key
// groups inline their children at the enclosing level.
func AttrToZapField(attr corelog.Attr) zap.Field {
	budget := maxGroupNodes
	return attrToZapFieldDepth(attr, 0, &budget)
}

func attrToZapFieldDepth(attr corelog.Attr, depth int, budget *int) zap.Field {
	*budget--
	if depth > maxGroupDepth || *budget < 0 {
		key := attr.Key
		if key == "" {
			key = "attr_error"
		}
		return zap.String(key, fmt.Sprintf("<unformattable %T: cyclic or deeper than %d levels>", attr.Value.Any(), maxGroupDepth))
	}
	v := attr.Value.Resolve()
	if v.Kind() != slog.KindGroup && attr.Key == "" {
		return zap.Skip()
	}
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
		group := v.Group()
		if len(group) == 0 {
			return zap.Skip()
		}
		fields := make([]zap.Field, 0, len(group))
		for _, a := range group {
			f := attrToZapFieldDepth(a, depth+1, budget)
			if f.Type != zapcore.SkipType {
				fields = append(fields, f)
			}
		}
		if len(fields) == 0 {
			return zap.Skip()
		}
		// Encode the group through an ObjectMarshaler rather than flattening it
		// into a map[string]any: a map goes through zap's reflection/JSON path,
		// which renders a nested time.Duration as its nanosecond integer, loses
		// error fields and collapses duplicate keys, making the output depend on
		// nesting depth.
		if attr.Key == "" {
			return zap.Inline(groupObject{fields: fields})
		}
		return zap.Object(attr.Key, groupObject{fields: fields})
	case slog.KindAny:
		if err, ok := v.Any().(error); ok && attr.Key == "error" {
			return zap.Error(err)
		}
		return zap.Any(attr.Key, v.Any())
	default:
		return zap.Any(attr.Key, fmt.Sprint(v.Any()))
	}
}

// groupObject encodes pre-resolved group fields with the same typed encoders
// used for top-level attrs, bounded by maxGroupDepth and maxGroupNodes.
type groupObject struct {
	fields []zap.Field
}

func (g groupObject) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for _, f := range g.fields {
		f.AddTo(enc)
	}
	return nil
}

func mapToSlogAttrs(attrs map[string]any) []slog.Attr {
	keys := slices.Sorted(maps.Keys(attrs))

	out := make([]slog.Attr, 0, len(attrs))
	for _, k := range keys {
		v := attrs[k]
		if sv, ok := v.(slog.Value); ok {
			out = append(out, slog.Attr{Key: k, Value: sv})
			continue
		}
		if k == "" {
			continue
		}
		out = append(out, slog.Any(k, v))
	}
	return out
}

// logLevelToZapLevel maps level to zap, clamping a level outside
// LevelDebug..LevelError to the nearest of them.
func logLevelToZapLevel(level corelog.Level) zapcore.Level {
	switch min(max(level, corelog.LevelDebug), corelog.LevelError) {
	case corelog.LevelDebug:
		return zapcore.DebugLevel
	case corelog.LevelInfo:
		return zapcore.InfoLevel
	case corelog.LevelWarn:
		return zapcore.WarnLevel
	default:
		return zapcore.ErrorLevel
	}
}

// logLevelToSlogLevel maps level to slog, clamping like logLevelToZapLevel.
func logLevelToSlogLevel(level corelog.Level) slog.Level {
	switch min(max(level, corelog.LevelDebug), corelog.LevelError) {
	case corelog.LevelDebug:
		return slog.LevelDebug
	case corelog.LevelInfo:
		return slog.LevelInfo
	case corelog.LevelWarn:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}
