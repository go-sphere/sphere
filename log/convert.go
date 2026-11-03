package log

import (
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func formatSlogValue(v slog.Value) string {
	return formatSlogValueDepth(v, 0)
}

// formatSlogValueDepth bounds the group recursion the same way formatAny
// bounds container recursion: a LogValuer can resolve to a group containing
// itself, and that cycle runs through KindGroup, never through formattable,
// so it must be cut here or the stack overflows.
func formatSlogValueDepth(v slog.Value, depth int) string {
	if depth > maxFormatDepth {
		return quoteIfNeeded(fmt.Sprintf("<unformattable %T: cyclic or deeper than %d levels>", v.Any(), maxFormatDepth))
	}
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return quoteIfNeeded(v.String())
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindDuration:
		return quoteIfNeeded(v.Duration().String())
	case slog.KindTime:
		return quoteIfNeeded(v.Time().Format(time.RFC3339Nano))
	case slog.KindGroup:
		return formatGroup(v.Group(), depth+1)
	case slog.KindAny:
		return formatAny(v.Any())
	default:
		return formatAny(v.Any())
	}
}

func formatGroup(attrs []slog.Attr, depth int) string {
	if len(attrs) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, quoteIfNeeded(a.Key)+"="+formatSlogValueDepth(a.Value, depth))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// maxFormatDepth bounds how deep formatAny will look before refusing to format
// a value.
const maxFormatDepth = 32

func formatAny(v any) string {
	// fmt.Sprint has no cycle detection, so a self-referential container recurses
	// until the goroutine stack is exhausted. That failure is fatal and cannot be
	// recovered — unlike a panic from a MarshalJSON, which the backend catches —
	// so it has to be prevented rather than handled. Values that are cyclic or
	// implausibly deep are reported instead of formatted.
	if !formattable(reflect.ValueOf(v), 0, make(map[containerKey]struct{})) {
		return quoteIfNeeded(fmt.Sprintf("<unformattable %T: cyclic or deeper than %d levels>", v, maxFormatDepth))
	}
	return quoteIfNeeded(fmt.Sprint(v))
}

// containerKey identifies a container on the current traversal path. For
// slices the length is part of the identity: rv.Pointer() is the backing
// array's base address, which an aliasing sub-slice (s[0:1] inside s) shares
// with its ancestor without forming a cycle, while a genuine self-reference
// has both the same base and the same length. Non-slice containers use -1.
type containerKey struct {
	ptr    uintptr
	length int
}

// formattable reports whether rv can be handed to fmt.Sprint without risking
// unbounded recursion. seen holds the container addresses on the current path,
// so a value reachable from itself is rejected while the same value appearing
// twice side by side is not.
func formattable(rv reflect.Value, depth int, seen map[containerKey]struct{}) bool {
	if depth > maxFormatDepth {
		return false
	}
	switch rv.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Interface:
		if rv.IsNil() {
			return true
		}
		return formattable(rv.Elem(), depth+1, seen)
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if rv.IsNil() {
			return true
		}
		key := containerKey{ptr: rv.Pointer(), length: -1}
		if rv.Kind() == reflect.Slice {
			key.length = rv.Len()
		}
		if _, ok := seen[key]; ok {
			return false
		}
		seen[key] = struct{}{}
		defer delete(seen, key)

		switch rv.Kind() {
		case reflect.Pointer:
			return formattable(rv.Elem(), depth+1, seen)
		case reflect.Map:
			iter := rv.MapRange()
			for iter.Next() {
				if !formattable(iter.Key(), depth+1, seen) || !formattable(iter.Value(), depth+1, seen) {
					return false
				}
			}
			return true
		default: // slice
			return formattableElems(rv, depth, seen)
		}
	case reflect.Array:
		return formattableElems(rv, depth, seen)
	case reflect.Struct:
		for i := range rv.NumField() {
			if !formattable(rv.Field(i), depth+1, seen) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func formattableElems(rv reflect.Value, depth int, seen map[containerKey]struct{}) bool {
	for i := range rv.Len() {
		if !formattable(rv.Index(i), depth+1, seen) {
			return false
		}
	}
	return true
}

func quoteIfNeeded(v string) string {
	if v == "" {
		return `""`
	}
	// Braces and commas delimit groups, and control characters (beyond the
	// whitespace already covered) would let logged data forge lines or inject
	// terminal escapes; quote them all so the encoding stays unambiguous.
	if strings.ContainsAny(v, " \t\n\r\"=,{}") || strings.ContainsFunc(v, unicode.IsControl) {
		return strconv.Quote(v)
	}
	return v
}

func levelString(level Level) string {
	switch level {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}
