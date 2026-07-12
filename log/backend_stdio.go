package log

import (
	"context"
	"io"
	"maps"
	"os"
	"runtime"
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

type StdioBackend struct {
	mu        sync.Mutex
	name      string
	attrs     map[string]any
	minLevel  Level
	addCaller bool
	// stackAt records the level at or above which stack traces should be
	// attached. Stack output is not yet implemented for the stdio backend, so
	// this currently has no visible effect; it no longer filters log entries.
	stackAt *Level
}

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
	line := b.buildLine(level, msg, caller, attrs)
	b.mu.Lock()
	_, _ = io.WriteString(writerForLevel(level), line)
	b.mu.Unlock()
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
	// AddStackAt controls stack trace attachment, not filtering. Stack output is
	// not yet implemented here, but record it so semantics stay aligned with the
	// zapx backend and future stack support.
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
