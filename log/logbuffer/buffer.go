// Package logbuffer is an in-memory, cursor-addressable log tail. Buffer is a
// log.Backend: mounted via log.InitWithBackends it captures every entry into a
// fixed-size ring with a monotonically increasing sequence number, and fans
// entries out to subscribers for live streaming (e.g. SSE).
//
// The sequence number is the correctness anchor. Subscribe atomically returns
// the backfill after a cursor together with the live channel, so there is no
// loss window between "history" and "live". A cursor that has fallen out of
// the ring, or entries dropped on a slow subscriber, are reported explicitly
// (Truncated, Dropped) instead of being silently lost.
//
// Sequence numbers restart at 1 on process restart. Pair resume cursors with
// Buffer.ID; Subscribe reports Reset when a cursor belongs to an earlier
// Buffer instance.
package logbuffer

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-sphere/sphere/log"
)

// Entry is one captured log record. Attrs carries the resolved structured
// fields (both preset With attrs and per-call attrs), so historical and live
// entries are identical in shape.
type Entry struct {
	Seq     uint64         `json:"seq"`
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Name    string         `json:"name,omitempty"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	level   log.Level
}

// DefaultSubscriberBuffer is the live-channel capacity used when
// SubscribeOptions.ChannelBuffer is zero.
const DefaultSubscriberBuffer = 256

// Subscription is a live tail handle returned by Subscribe.
type Subscription struct {
	// Backfill contains the ring entries selected by SubscribeOptions,
	// captured atomically with the live registration: the first entry on C
	// is always the successor of the last backfill entry.
	Backfill []Entry
	// Truncated reports that entries between FromSeq and the oldest ring
	// entry no longer exist (the ring wrapped past the cursor). Backfill
	// then starts at the oldest retained entry.
	Truncated bool
	// Reset reports that SubscribeOptions.StreamID named an earlier Buffer
	// instance. Backfill then follows the fresh-subscription TailLimit rule,
	// and the caller must replace its local cursor with this Buffer's ID and
	// the sequence numbers in Backfill/C.
	Reset bool
	// C delivers live entries. It is never closed; consumers stop by
	// selecting on their own context and calling Cancel.
	C <-chan Entry

	ch      chan Entry
	level   log.Level
	dropped atomic.Uint64
	cancel  func()
}

// Dropped returns the number of live entries discarded because C was full.
// A non-zero value means there is a gap after the last received entry;
// resubscribe from the last seen sequence to recover what the ring retains.
func (s *Subscription) Dropped() uint64 {
	return s.dropped.Load()
}

// Cancel deregisters the subscription. It is idempotent. Entries already
// buffered in C remain readable; C is not closed.
func (s *Subscription) Cancel() {
	s.cancel()
}

// SubscribeOptions selects the backfill and filtering for a Subscription.
type SubscribeOptions struct {
	// StreamID is the Buffer ID associated with FromSeq. When it is non-empty
	// and differs from Buffer.ID, the process (or Buffer) has restarted: the
	// cursor is reset and Subscription.Reset is true. An empty StreamID keeps
	// backward compatibility with sequence-only callers.
	StreamID string
	// FromSeq is the resume cursor: the last sequence number the caller has
	// already seen. Backfill contains entries with Seq > FromSeq. Zero means
	// "no continuity claim": backfill is the newest TailLimit entries and
	// Truncated stays false.
	FromSeq uint64
	// TailLimit caps the backfill size when FromSeq is zero. Zero means no
	// backfill for fresh subscribers; negative means the whole ring.
	TailLimit int
	// MinLevel drops entries below the given level from both backfill and
	// the live channel.
	MinLevel log.Level
	// ChannelBuffer is the live-channel capacity; zero uses
	// DefaultSubscriberBuffer. When the channel is full new entries are
	// dropped and counted (see Dropped) instead of blocking the log path.
	ChannelBuffer int
}

// Buffer is the ring plus its subscriber set. It implements log.Backend; use
// New and pass it to log.InitWithBackends alongside the output backends.
type Buffer struct {
	mu      sync.Mutex
	id      string
	ring    []Entry
	count   int
	nextSeq uint64 // seq assigned to the next entry; first entry gets 1
	subs    map[*Subscription]struct{}

	// Backend view configuration (root view; With returns derived views).
	name     string
	attrs    map[string]any
	minLevel log.Level
}

// New returns a Buffer retaining the last capacity entries (minimum 1), then
// applies backend options. log.WithMinLevel filters what enters the ring.
func New(capacity int, options ...log.Option) *Buffer {
	capacity = max(capacity, 1)
	b := &Buffer{
		id:      rand.Text(),
		ring:    make([]Entry, capacity),
		nextSeq: 1,
		subs:    make(map[*Subscription]struct{}),
	}
	opts := log.NewOptions(options...)
	b.name = opts.Name
	b.attrs = opts.Attrs
	if opts.MinLevel != nil {
		b.minLevel = *opts.MinLevel
	} else {
		b.minLevel = log.LevelDebug
	}
	return b
}

// ID identifies this Buffer instance. It changes whenever New is called, so
// clients can distinguish a valid resume cursor from a sequence number issued
// by an earlier process instance.
func (b *Buffer) ID() string {
	return b.id
}

// Log implements log.Backend: it records the entry into the ring and fans it
// out to live subscribers without ever blocking on a slow one.
func (b *Buffer) Log(_ context.Context, level log.Level, msg string, attrs ...log.Attr) {
	if level < b.minLevel {
		return
	}
	b.publish(level, msg, b.name, b.attrs, attrs)
}

// Sync implements log.Backend; the ring is memory-only so it is a no-op.
func (b *Buffer) Sync() error { return nil }

// With implements log.Backend. The derived backend shares this Buffer's ring
// and subscribers — only the preset attrs, name, and minimum level differ —
// so sub-loggers feed the same tail.
func (b *Buffer) With(options ...log.Option) log.Backend {
	opts := log.NewOptions(options...)
	v := &view{
		buf:      b,
		name:     b.name,
		attrs:    b.attrs,
		minLevel: b.minLevel,
	}
	if opts.Name != "" {
		v.name = joinName(v.name, opts.Name)
	}
	if opts.MinLevel != nil {
		v.minLevel = *opts.MinLevel
	}
	if len(opts.Attrs) > 0 {
		merged := make(map[string]any, len(b.attrs)+len(opts.Attrs))
		maps.Copy(merged, b.attrs)
		maps.Copy(merged, opts.Attrs)
		v.attrs = merged
	}
	return v
}

// view is a derived Backend sharing the parent Buffer's ring.
type view struct {
	buf      *Buffer
	name     string
	attrs    map[string]any
	minLevel log.Level
}

func (v *view) Log(_ context.Context, level log.Level, msg string, attrs ...log.Attr) {
	if level < v.minLevel {
		return
	}
	v.buf.publish(level, msg, v.name, v.attrs, attrs)
}

func (v *view) Sync() error { return nil }

func (v *view) With(options ...log.Option) log.Backend {
	opts := log.NewOptions(options...)
	next := &view{buf: v.buf, name: v.name, attrs: v.attrs, minLevel: v.minLevel}
	if opts.Name != "" {
		next.name = joinName(next.name, opts.Name)
	}
	if opts.MinLevel != nil {
		next.minLevel = *opts.MinLevel
	}
	if len(opts.Attrs) > 0 {
		merged := make(map[string]any, len(v.attrs)+len(opts.Attrs))
		maps.Copy(merged, v.attrs)
		maps.Copy(merged, opts.Attrs)
		next.attrs = merged
	}
	return next
}

// publish records one already-level-filtered entry (callers apply their own
// minLevel first) and fans it out to matching subscribers.
func (b *Buffer) publish(level log.Level, msg, name string, preset map[string]any, attrs []log.Attr) {
	entry := Entry{
		Time:    time.Now(),
		Level:   LevelString(level),
		Name:    name,
		Message: msg,
		Attrs:   mergeAttrs(preset, attrs),
		level:   level,
	}

	b.mu.Lock()
	entry.Seq = b.nextSeq
	b.nextSeq++
	b.ring[int((entry.Seq-1)%uint64(len(b.ring)))] = entry
	if b.count < len(b.ring) {
		b.count++
	}
	for sub := range b.subs {
		if level < sub.level {
			continue
		}
		select {
		case sub.ch <- entry:
		default:
			// Never block the process's log path on a slow consumer; the
			// drop is observable via Dropped and recoverable via the ring.
			sub.dropped.Add(1)
		}
	}
	b.mu.Unlock()
}

// Subscribe atomically captures the requested backfill and registers a live
// channel, guaranteeing gap-free continuity between the two. Callers must
// Cancel the subscription when done.
func (b *Buffer) Subscribe(opts SubscribeOptions) *Subscription {
	buffer := opts.ChannelBuffer
	if buffer <= 0 {
		buffer = DefaultSubscriberBuffer
	}
	sub := &Subscription{
		ch:    make(chan Entry, buffer),
		level: opts.MinLevel,
	}
	sub.C = sub.ch
	sub.cancel = sync.OnceFunc(func() {
		b.mu.Lock()
		delete(b.subs, sub)
		b.mu.Unlock()
	})

	b.mu.Lock()
	defer b.mu.Unlock()
	if opts.StreamID != "" && opts.StreamID != b.id {
		sub.Reset = true
		if opts.TailLimit != 0 {
			sub.Backfill = b.collectLocked(0, opts.TailLimit, opts.MinLevel)
		}
	} else if opts.FromSeq > 0 {
		oldest := b.oldestSeqLocked()
		if oldest > opts.FromSeq+1 && b.count > 0 {
			sub.Truncated = true
		}
		sub.Backfill = b.collectLocked(opts.FromSeq, -1, opts.MinLevel)
	} else if opts.TailLimit != 0 {
		sub.Backfill = b.collectLocked(0, opts.TailLimit, opts.MinLevel)
	}
	b.subs[sub] = struct{}{}
	return sub
}

// History returns up to limit entries with Seq < beforeSeq (newest last) at
// or above minLevel, for paging backwards through the retained tail. A
// beforeSeq of zero starts from the newest entry. more reports whether another
// matching entry remains before this page.
func (b *Buffer) History(beforeSeq uint64, limit int, minLevel log.Level) (entries []Entry, more bool) {
	if limit <= 0 {
		limit = 100
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	oldest := b.oldestSeqLocked()
	end := b.nextSeq // exclusive
	if beforeSeq > 0 && beforeSeq < end {
		end = beforeSeq
	}
	for seq := end - 1; seq >= oldest && seq > 0; seq-- {
		e := b.ring[int((seq-1)%uint64(len(b.ring)))]
		if e.level >= minLevel {
			if len(entries) == limit {
				more = true
				break
			}
			entries = append(entries, e)
		}
		if seq == oldest {
			break
		}
	}
	slices.Reverse(entries)
	return entries, more
}

// LatestSeq returns the sequence number of the newest retained entry (zero
// when the ring is empty).
func (b *Buffer) LatestSeq() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.nextSeq - 1
}

// oldestSeqLocked returns the oldest retained sequence number, or nextSeq
// when the ring is empty.
func (b *Buffer) oldestSeqLocked() uint64 {
	if b.count == 0 {
		return b.nextSeq
	}
	return b.nextSeq - uint64(b.count)
}

// collectLocked gathers entries with Seq > fromSeq at or above minLevel in
// ascending order. A non-negative limit keeps only the newest limit entries.
func (b *Buffer) collectLocked(fromSeq uint64, limit int, minLevel log.Level) []Entry {
	if b.count == 0 || limit == 0 {
		return nil
	}
	oldest := b.oldestSeqLocked()
	start := max(fromSeq+1, oldest)
	var out []Entry
	for seq := start; seq < b.nextSeq; seq++ {
		e := b.ring[int((seq-1)%uint64(len(b.ring)))]
		if e.level >= minLevel {
			out = append(out, e)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// mergeAttrs resolves preset attrs and per-call slog attrs into one map.
// Rendering arbitrary attr values can panic (custom Stringer/LogValuer); a
// panicking value degrades to an attr_error entry, mirroring StdioBackend.
func mergeAttrs(preset map[string]any, attrs []log.Attr) (out map[string]any) {
	if len(preset) == 0 && len(attrs) == 0 {
		return nil
	}
	out = make(map[string]any, len(preset)+len(attrs))
	maps.Copy(out, preset)
	defer func() {
		if r := recover(); r != nil {
			out["attr_error"] = fmt.Sprint(r)
		}
	}()
	for _, attr := range attrs {
		addAttr(out, attr)
	}
	return out
}

// addAttr resolves one slog attr into dst, flattening groups into nested maps.
func addAttr(dst map[string]any, attr log.Attr) {
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		group := value.Group()
		if attr.Key == "" {
			for _, ga := range group {
				addAttr(dst, ga)
			}
			return
		}
		nested := make(map[string]any, len(group))
		for _, ga := range group {
			addAttr(nested, ga)
		}
		dst[attr.Key] = nested
		return
	}
	if attr.Key == "" {
		return
	}
	dst[attr.Key] = value.Any()
}

// LevelString renders a log.Level as its lowercase name.
func LevelString(level log.Level) string {
	switch level {
	case log.LevelDebug:
		return "debug"
	case log.LevelInfo:
		return "info"
	case log.LevelWarn:
		return "warn"
	case log.LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// ParseLevel converts a level name to a log.Level. Unknown or empty input
// reports ok=false with LevelDebug (the permissive default for filters).
func ParseLevel(s string) (log.Level, bool) {
	switch s {
	case "debug":
		return log.LevelDebug, true
	case "info":
		return log.LevelInfo, true
	case "warn", "warning":
		return log.LevelWarn, true
	case "error":
		return log.LevelError, true
	default:
		return log.LevelDebug, false
	}
}

func joinName(base, next string) string {
	switch {
	case base == "":
		return next
	case next == "":
		return base
	default:
		return base + "." + next
	}
}
