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
	budget := maxFormatNodes
	return formatSlogValueDepth(v, 0, &budget)
}

// formatSlogValueDepth bounds the group recursion the same way formatAny
// bounds container recursion: a LogValuer can resolve to a group containing
// itself, and that cycle runs through KindGroup, never through formatAny, so
// it must be cut here or the stack overflows. budget caps the values rendered
// in total, because a group that refers to itself more than once branches at
// every level and the depth bound alone would allow exponential work.
func formatSlogValueDepth(v slog.Value, depth int, budget *int) string {
	*budget--
	if depth > maxFormatDepth || *budget < 0 {
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
		return formatGroup(v.Group(), depth+1, budget)
	case slog.KindAny:
		return formatAny(v.Any())
	default:
		return formatAny(v.Any())
	}
}

func formatGroup(attrs []slog.Attr, depth int, budget *int) string {
	if len(attrs) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, quoteIfNeeded(a.Key)+"="+formatSlogValueDepth(a.Value, depth, budget))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// maxFormatNodes bounds how many values one formatSlogValue call renders.
const maxFormatNodes = 4096

// maxFormatDepth bounds how deeply formatSlogValue nests groups before
// refusing to format them.
const maxFormatDepth = 32

// maxFormattableDepth bounds how deep formatAny will look before refusing to
// format a value. It counts reflection levels, and each %w layer of an error
// chain costs three of them (interface, pointer, struct), so it is set well
// above maxFormatDepth to keep real error chains printable. Cycles are caught
// by identity, not by this bound.
const maxFormattableDepth = 128

func formatAny(v any) string {
	// fmt.Sprint has no cycle detection, so a self-referential container recurses
	// until the goroutine stack is exhausted. That failure is fatal and cannot be
	// recovered — unlike a panic from a MarshalJSON, which the backend catches —
	// so it has to be prevented rather than handled. Values that are cyclic or
	// implausibly deep are reported instead of formatted.
	w := formatWalk{
		onPath:   make(map[containerKey]struct{}),
		verified: make(map[containerKey]int),
	}
	if _, ok := w.walk(reflect.ValueOf(v), 0); !ok {
		return quoteIfNeeded(fmt.Sprintf("<unformattable %T: cyclic or deeper than %d levels>", v, maxFormattableDepth))
	}
	return quoteIfNeeded(fmt.Sprint(v))
}

// containerKey identifies a container. For slices the length is part of the
// identity: rv.Pointer() is the backing array's base address, which an
// aliasing sub-slice (s[0:1] inside s) shares with its ancestor without forming
// a cycle, while a genuine self-reference has both the same base and the same
// length. Non-slice containers use -1. The type separates a pointer to a struct
// from a pointer to its first field, which share an address.
type containerKey struct {
	ptr    uintptr
	length int
	typ    reflect.Type
}

// formatWalk checks whether a value can be handed to fmt.Sprint without risking
// unbounded recursion. onPath holds the containers on the current path, so a
// value reachable from itself is rejected while the same value appearing twice
// side by side is not. verified maps each container already found acyclic to
// the number of levels below it, so a shared subtree is walked once instead of
// once per path that reaches it.
type formatWalk struct {
	onPath   map[containerKey]struct{}
	verified map[containerKey]int
}

// walk reports whether rv, found depth levels down, is formattable and, if so,
// how many levels lie below it.
func (w *formatWalk) walk(rv reflect.Value, depth int) (height int, ok bool) {
	if depth > maxFormattableDepth {
		return 0, false
	}
	switch rv.Kind() {
	case reflect.Invalid:
		return 0, true
	case reflect.Interface:
		if rv.IsNil() {
			return 0, true
		}
		return w.child(rv.Elem(), depth)
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if rv.IsNil() {
			return 0, true
		}
		key := containerKey{ptr: rv.Pointer(), length: -1, typ: rv.Type()}
		if rv.Kind() == reflect.Slice {
			key.length = rv.Len()
		}
		if _, ok := w.onPath[key]; ok {
			return 0, false
		}
		if h, ok := w.verified[key]; ok {
			return h, depth+h <= maxFormattableDepth
		}
		w.onPath[key] = struct{}{}
		defer delete(w.onPath, key)

		switch rv.Kind() {
		case reflect.Pointer:
			height, ok = w.child(rv.Elem(), depth)
		case reflect.Map:
			ok = true
			iter := rv.MapRange()
			for ok && iter.Next() {
				height, ok = w.maxChild(height, iter.Key(), depth)
				if ok {
					height, ok = w.maxChild(height, iter.Value(), depth)
				}
			}
		default: // slice
			height, ok = w.elems(rv, depth)
		}
		if ok {
			w.verified[key] = height
		}
		return height, ok
	case reflect.Array:
		return w.elems(rv, depth)
	case reflect.Struct:
		ok = true
		for i := 0; ok && i < rv.NumField(); i++ {
			height, ok = w.maxChild(height, rv.Field(i), depth)
		}
		return height, ok
	default:
		return 0, true
	}
}

// child walks rv as the child of a value at depth and returns its height as
// seen from that parent.
func (w *formatWalk) child(rv reflect.Value, depth int) (int, bool) {
	h, ok := w.walk(rv, depth+1)
	return h + 1, ok
}

// maxChild walks rv as a child of a value at depth and folds its height into
// height.
func (w *formatWalk) maxChild(height int, rv reflect.Value, depth int) (int, bool) {
	h, ok := w.child(rv, depth)
	return max(height, h), ok
}

func (w *formatWalk) elems(rv reflect.Value, depth int) (height int, ok bool) {
	ok = true
	for i := 0; ok && i < rv.Len(); i++ {
		height, ok = w.maxChild(height, rv.Index(i), depth)
	}
	return height, ok
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
