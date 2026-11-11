package log

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sphere/sphere/log/internal/callsite"
)

// StdioBackend writes logfmt lines with a UTC RFC3339 timestamp. Entries
// below the minimum level are dropped. LevelError and above go to stderr;
// other levels go to stdout. It honors WithMinLevel, WithName, WithAttrs,
// AddCaller/DisableCaller, and WithStackAt. The caller is the first stack
// frame outside the log package tree, so it stays the user's call site through
// MultiBackend and WrapBackendWithContextMerge.
//
// The zero value is usable and logs every level without a name or caller;
// it is the initial global backend. A StdioBackend is safe for concurrent
// use and has nothing to close. A panic while rendering an attribute value
// is recovered and logged as an attr_error field instead.
type StdioBackend struct {
	mu    sync.Mutex
	name  string
	attrs map[string]any
	// attrsText is attrs rendered once, in key order, as " key=value" pairs.
	attrsText string
	minLevel  Level
	addCaller bool
	// stackAt records the level at or above which a "stack" attribute carrying
	// the current goroutine's stack trace is attached. It is not a level filter;
	// MinLevel is the only thing that decides whether an entry is emitted.
	stackAt *Level
}

// NewStdioBackend returns a StdioBackend with min level debug, then applies
// options. WithMinLevel is honored.
func NewStdioBackend(options ...Option) *StdioBackend {
	b := &StdioBackend{
		attrs:    make(map[string]any),
		minLevel: LevelDebug,
	}
	return b.apply(options...)
}

// Log writes one line for level and msg unless level is below the minimum. A
// level outside LevelDebug..LevelError is clamped to the nearest of them first.
// The context is ignored.
func (b *StdioBackend) Log(_ context.Context, level Level, msg string, attrs ...Attr) {
	level = min(max(level, LevelDebug), LevelError)
	if level < b.minLevel {
		return
	}
	var caller string
	if b.addCaller {
		caller = callerLocation()
	}
	if b.stackAt != nil && level >= *b.stackAt {
		attrs = append(attrs[:len(attrs):len(attrs)], String("stack", string(debug.Stack())))
	}
	line := b.buildLineSafely(level, msg, caller, attrs)
	b.mu.Lock()
	_, _ = io.WriteString(writerForLevel(level), line)
	b.mu.Unlock()
}

// buildLineSafely renders the line, degrading to a diagnostic when rendering an
// attribute panics. Attribute values are arbitrary caller data — a Stringer or
// MarshalJSON of their own — so formatting one can panic; letting that escape
// would mean a log statement aborts the goroutine that made it, and log
// statements sit on error paths where that is the worst possible moment.
func (b *StdioBackend) buildLineSafely(level Level, msg, caller string, attrs []Attr) (line string) {
	defer func() {
		if r := recover(); r != nil {
			// The fallback formats r itself, which can panic again — guard it
			// too, degrading to a minimal line instead of letting a second
			// panic escape past the consumed recover.
			defer func() {
				if recover() != nil {
					line = "level=" + levelString(level) + " msg=" + quoteIfNeeded(msg) + " attr_error=unprintable\n"
				}
			}()
			line = b.buildLine(level, msg, caller, []Attr{
				String("attr_error", fmt.Sprint(r)),
			})
		}
	}()
	return b.buildLine(level, msg, caller, attrs)
}

// Sync is a no-op; lines are written unbuffered. It always returns nil.
func (b *StdioBackend) Sync() error {
	return nil
}

// With returns a copy with options applied; the receiver is unchanged.
// Names are joined with "." and attrs are merged, new keys winning.
func (b *StdioBackend) With(options ...Option) Backend {
	return b.clone().apply(options...)
}

func (b *StdioBackend) clone() *StdioBackend {
	attrs := make(map[string]any, len(b.attrs))
	maps.Copy(attrs, b.attrs)
	return &StdioBackend{
		name:      b.name,
		attrs:     attrs,
		attrsText: b.attrsText,
		minLevel:  b.minLevel,
		addCaller: b.addCaller,
		stackAt:   b.stackAt,
	}
}

func (b *StdioBackend) apply(options ...Option) *StdioBackend {
	if len(options) == 0 {
		return b
	}
	o := NewOptions(options...)
	if o.Name != "" {
		if b.name == "" {
			b.name = o.Name
		} else {
			b.name = b.name + "." + o.Name
		}
	}
	switch o.AddCaller {
	case AddCallerStatusEnable:
		b.addCaller = true
	case AddCallerStatusDisable:
		b.addCaller = false
	default:
		break
	}
	// AddStackAt controls stack trace attachment, not filtering, matching the
	// zapx backend's zap.AddStacktrace semantics.
	if o.AddStackAt != nil {
		l := min(max(*o.AddStackAt, LevelDebug), LevelError)
		b.stackAt = &l
	}
	// MinLevel is the only level filter for this backend.
	if o.MinLevel != nil {
		b.minLevel = *o.MinLevel
	}
	if len(o.Attrs) > 0 {
		if b.attrs == nil {
			b.attrs = make(map[string]any, len(o.Attrs))
		}
		maps.Copy(b.attrs, o.Attrs)
		b.attrsText = renderAttrs(b.attrs)
	}
	return b
}

// renderAttrs renders backend-level attrs in key order as " key=value" pairs.
// A value that panics while rendering degrades to an attr_error field, as it
// does in Log.
func renderAttrs(attrs map[string]any) (text string) {
	defer func() {
		if recover() != nil {
			text = " attr_error=unprintable"
		}
	}()
	var sb strings.Builder
	for _, k := range slices.Sorted(maps.Keys(attrs)) {
		sb.WriteByte(' ')
		sb.WriteString(quoteIfNeeded(k))
		sb.WriteByte('=')
		sb.WriteString(formatAny(attrs[k]))
	}
	return sb.String()
}

func (b *StdioBackend) buildLine(level Level, msg string, caller string, attrs []Attr) string {
	var sb strings.Builder
	sb.Grow(128)
	sb.WriteString(time.Now().UTC().Format(time.RFC3339))
	sb.WriteString(" level=")
	sb.WriteString(levelString(level))
	if b.name != "" {
		sb.WriteString(" logger=")
		sb.WriteString(quoteIfNeeded(b.name))
	}
	if caller != "" {
		sb.WriteString(" caller=")
		sb.WriteString(quoteIfNeeded(caller))
	}
	sb.WriteString(" msg=")
	sb.WriteString(quoteIfNeeded(msg))
	sb.WriteString(b.attrsText)
	for _, a := range attrs {
		sb.WriteByte(' ')
		sb.WriteString(quoteIfNeeded(a.Key))
		sb.WriteByte('=')
		sb.WriteString(formatSlogValue(a.Value))
	}
	sb.WriteByte('\n')
	return sb.String()
}

func writerForLevel(level Level) io.Writer {
	if level >= LevelError {
		return os.Stderr
	}
	return os.Stdout
}

// callerLocation returns "file:line" for the first stack frame outside the log
// package tree, using only the base file name. It returns "" when unavailable.
func callerLocation() string {
	frame, ok := callsite.Frame()
	if !ok || frame.File == "" {
		return ""
	}
	file := frame.File
	if idx := strings.LastIndexByte(file, '/'); idx >= 0 {
		file = file[idx+1:]
	}
	return file + ":" + strconv.Itoa(frame.Line)
}
