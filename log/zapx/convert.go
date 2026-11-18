package zapx

import (
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"

	corelog "github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/internal/safefmt"
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
		fields = append(fields, safeAnyField(k, v))
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
		raw := v.Any()
		if msg, ok := safefmt.Check(raw); !ok {
			return zap.String(attr.Key, msg)
		}
		if err, ok := raw.(error); ok && attr.Key == "error" {
			return zap.Error(wrapSafeError(err))
		}
		return safeCheckedAnyField(attr.Key, raw)
	default:
		return zap.Any(attr.Key, safefmt.Sprint(v.Any()))
	}
}

func safeAnyField(key string, raw any) zap.Field {
	if msg, ok := safefmt.Check(raw); !ok {
		return zap.String(key, msg)
	}
	return safeCheckedAnyField(key, raw)
}

func safeCheckedAnyField(key string, raw any) zap.Field {
	switch v := raw.(type) {
	case zapcore.ObjectMarshaler, zapcore.ArrayMarshaler, []zap.Field:
		return zap.Any(key, raw)
	case error:
		return zap.NamedError(key, wrapSafeError(v))
	case []error:
		wrapped := make([]error, len(v))
		for i, err := range v {
			wrapped[i] = wrapSafeError(err)
		}
		return zap.Errors(key, wrapped)
	case fmt.Stringer:
		// Guard against Stringer panicking with a cyclic value that zapcore
		// would otherwise format via fmt.Errorf("PANIC=%v", rerr).
		if _, isObj := raw.(zapcore.ObjectMarshaler); !isObj && !isTypedNil(raw) {
			return zap.Stringer(key, safeStringer{s: v})
		}
		return zap.Any(key, raw)
	default:
		return zap.Any(key, raw)
	}
}

func isTypedNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

func wrapSafeError(err error) error {
	if err == nil || isTypedNil(err) {
		return err
	}
	return safeError{err: err}
}

type safeError struct {
	err error
}

func (e safeError) Error() string {
	msg, panicked := safeStringCall(e.err.Error)
	if panicked {
		panic(msg)
	}
	return msg
}

func (e safeError) Unwrap() error {
	return e.err
}

type safeStringer struct {
	s fmt.Stringer
}

func (s safeStringer) String() string {
	msg, panicked := safeStringCall(s.s.String)
	if panicked {
		panic(msg)
	}
	return msg
}

func safeStringCall(fn func() string) (out string, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			out = formatPanicSafely(r)
		}
	}()
	return fn(), false
}

func formatPanicSafely(r any) (out string) {
	defer func() {
		if recover() != nil {
			out = "unprintable"
		}
	}()
	return safefmt.Sprint(r)
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
