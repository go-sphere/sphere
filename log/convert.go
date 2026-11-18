package log

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-sphere/sphere/log/internal/safefmt"
)

func appendSlogAttr(sb *strings.Builder, a slog.Attr) {
	budget := maxFormatNodes
	walkSlogAttr(a, 0, &budget, func(key, rendered string) {
		sb.WriteByte(' ')
		sb.WriteString(quoteIfNeeded(key))
		sb.WriteByte('=')
		sb.WriteString(rendered)
	})
}

// walkSlogAttr formats a according to slog rules: non-group attrs with an empty
// key and empty groups are skipped, and groups with an empty key inline their
// members into the enclosing level. Group traversal is bounded by maxFormatDepth
// and maxFormatNodes just like formatSlogValueDepth.
func walkSlogAttr(a slog.Attr, depth int, budget *int, emit func(key, rendered string)) {
	*budget--
	if depth > maxFormatDepth || *budget < 0 {
		key := a.Key
		if key == "" {
			key = "attr_error"
		}
		emit(key, quoteIfNeeded(fmt.Sprintf("<unformattable %T: cyclic or deeper than %d levels>", a.Value.Any(), maxFormatDepth)))
		return
	}
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		group := v.Group()
		if a.Key == "" {
			for _, ga := range group {
				walkSlogAttr(ga, depth+1, budget, emit)
			}
			return
		}
		formatted, ok := formatGroup(group, depth+1, budget)
		if !ok {
			return
		}
		emit(a.Key, formatted)
		return
	}
	if a.Key == "" {
		return
	}
	emit(a.Key, formatResolvedSlogValue(v, depth, budget))
}

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
	return formatResolvedSlogValue(v.Resolve(), depth, budget)
}

func formatResolvedSlogValue(v slog.Value, depth int, budget *int) string {
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
		formatted, ok := formatGroup(v.Group(), depth+1, budget)
		if !ok {
			return "{}"
		}
		return formatted
	default:
		return formatAny(v.Any())
	}
}

func formatGroup(attrs []slog.Attr, depth int, budget *int) (string, bool) {
	if len(attrs) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		walkSlogAttr(a, depth, budget, func(key, rendered string) {
			parts = append(parts, quoteIfNeeded(key)+"="+rendered)
		})
	}
	if len(parts) == 0 {
		return "", false
	}
	return "{" + strings.Join(parts, ",") + "}", true
}

// maxFormatNodes bounds how many values one formatSlogValue call renders.
const maxFormatNodes = 4096

// maxFormatDepth bounds how deeply formatSlogValue nests groups before
// refusing to format them.
const maxFormatDepth = 32

func formatAny(v any) string {
	return quoteIfNeeded(safefmt.Sprint(v))
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
