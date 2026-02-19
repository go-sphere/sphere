package log

import (
	"context"
	"io"
	"maps"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type StdioBackend struct {
	mu       sync.Mutex
	name     string
	attrs    map[string]any
	minLevel Level
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
	line := b.buildLine(level, msg, attrs)
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
		name:     b.name,
		attrs:    attrs,
		minLevel: b.minLevel,
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
	if o.AddStackAt != nil {
		b.minLevel = *o.AddStackAt
	}
	if len(o.Attrs) > 0 {
		if b.attrs == nil {
			b.attrs = make(map[string]any, len(o.Attrs))
		}
		maps.Copy(b.attrs, o.Attrs)
	}
	return b
}

func (b *StdioBackend) buildLine(level Level, msg string, attrs []Attr) string {
	var sb strings.Builder
	sb.Grow(128)
	sb.WriteString(time.Now().UTC().Format(time.RFC3339))
	sb.WriteString(" level=")
	sb.WriteString(levelString(level))
	if b.name != "" {
		sb.WriteString(" logger=")
		sb.WriteString(quoteIfNeeded(b.name))
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
