package log

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// stdioCallerSkip is the number of stack frames between callerLocation and the
// user code that invoked a logging method through the standard logger facade
// (callerLocation -> Log -> facade method -> caller). Because wrapper backends
// such as MultiBackend add extra frames, the reported location is best-effort.
const stdioCallerSkip = 3

// StdioBackend writes logfmt lines with a UTC RFC3339 timestamp. Entries
// below minLevel are dropped. LevelError and above go to stderr; other
// levels go to stdout. It honors log.WithMinLevel.
type StdioBackend struct {
	mu        sync.Mutex
	name      string
	attrs     map[string]any
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

func (b *StdioBackend) Log(_ context.Context, level Level, msg string, attrs ...Attr) {
	if level < b.minLevel {
		return
	}
	var caller string
	if b.addCaller {
		caller = callerLocation(stdioCallerSkip)
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
			line = b.buildLine(level, msg, caller, []Attr{
				String("attr_error", fmt.Sprint(r)),
			})
		}
	}()
	return b.buildLine(level, msg, caller, attrs)
}

func (b *StdioBackend) Sync() error {
	return nil
}

func (b *StdioBackend) With(options ...Option) Backend {
	return b.clone().apply(options...)
}

func (b *StdioBackend) clone() *StdioBackend {
	attrs := make(map[string]any, len(b.attrs))
	maps.Copy(attrs, b.attrs)
	return &StdioBackend{
		name:      b.name,
		attrs:     attrs,
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
		l := *o.AddStackAt
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
	}
	return b
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

	// Stable ordering for backend-level attrs improves testability and readability.
	if len(b.attrs) > 0 {
		keys := make([]string, 0, len(b.attrs))
		for k := range b.attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteByte(' ')
			sb.WriteString(k)
			sb.WriteByte('=')
			sb.WriteString(formatAny(b.attrs[k]))
		}
	}
	for _, a := range attrs {
		sb.WriteByte(' ')
		sb.WriteString(a.Key)
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

// callerLocation returns a "file:line" string for the caller at the given stack
// skip depth, using only the base file name. It returns "" when unavailable.
func callerLocation(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	if idx := strings.LastIndexByte(file, '/'); idx >= 0 {
		file = file[idx+1:]
	}
	return file + ":" + strconv.Itoa(line)
}
