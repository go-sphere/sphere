package log

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"
)

func formatSlogValue(v slog.Value) string {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return quoteIfNeeded(v.String())
	case slog.KindInt64:
		return fmt.Sprintf("%d", v.Int64())
	case slog.KindUint64:
		return fmt.Sprintf("%d", v.Uint64())
	case slog.KindFloat64:
		return fmt.Sprintf("%g", v.Float64())
	case slog.KindBool:
		return fmt.Sprintf("%t", v.Bool())
	case slog.KindDuration:
		return quoteIfNeeded(v.Duration().String())
	case slog.KindTime:
		return quoteIfNeeded(v.Time().Format(time.RFC3339Nano))
	case slog.KindGroup:
		return formatGroup(v.Group())
	case slog.KindAny:
		return formatAny(v.Any())
	default:
		return formatAny(v.Any())
	}
}

func formatGroup(attrs []slog.Attr) string {
	if len(attrs) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, a.Key+"="+formatSlogValue(a.Value))
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
	if !formattable(reflect.ValueOf(v), 0, make(map[uintptr]struct{})) {
		return quoteIfNeeded(fmt.Sprintf("<unformattable %T: cyclic or deeper than %d levels>", v, maxFormatDepth))
	}
	return quoteIfNeeded(fmt.Sprint(v))
}

// formattable reports whether rv can be handed to fmt.Sprint without risking
// unbounded recursion. seen holds the container addresses on the current path,
// so a value reachable from itself is rejected while the same value appearing
// twice side by side is not.
func formattable(rv reflect.Value, depth int, seen map[uintptr]struct{}) bool {
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
		addr := rv.Pointer()
		if _, ok := seen[addr]; ok {
			return false
		}
		seen[addr] = struct{}{}
		defer delete(seen, addr)

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

func formattableElems(rv reflect.Value, depth int, seen map[uintptr]struct{}) bool {
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
	if strings.ContainsAny(v, " \t\n\r\"=") {
		return fmt.Sprintf("%q", v)
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
