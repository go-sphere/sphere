package logbuffer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/sphere/log"
)

var _ log.Backend = (*Buffer)(nil)

func TestBufferBackendAndHistory(t *testing.T) {
	b := New(3,
		log.WithName("app"),
		log.WithMinLevel(log.LevelInfo),
		log.WithAttrs(map[string]any{"version": "v1"}),
	)

	b.Log(context.Background(), log.LevelDebug, "filtered")
	b.Log(context.Background(), log.LevelInfo, "started", log.String("component", "api"))
	b.Log(context.Background(), log.LevelWarn, "slow")
	b.Log(context.Background(), log.LevelError, "failed")
	b.Log(context.Background(), log.LevelInfo, "ready")

	entries, more := b.History(0, 10, log.LevelDebug)
	if more {
		t.Fatal("History more = true, want false after reading the whole retained ring")
	}
	wantSeqs(t, entries, 2, 3, 4)
	if got := entries[2].Name; got != "app" {
		t.Fatalf("Name = %q, want app", got)
	}
	if got := entries[2].Attrs["version"]; got != "v1" {
		t.Fatalf("version attr = %v, want v1", got)
	}
	if got := b.LatestSeq(); got != 4 {
		t.Fatalf("LatestSeq = %d, want 4", got)
	}
}

func TestBufferWithAccumulatesNameAndAttrs(t *testing.T) {
	b := New(4, log.WithName("root"), log.WithAttrs(map[string]any{"a": 1, "same": "root"}))
	child := b.With(
		log.WithName("worker"),
		log.WithAttrs(map[string]any{"b": 2, "same": "child"}),
	).With(log.WithName("sync"))

	child.Log(context.Background(), log.LevelInfo, "run", log.String("same", "entry"))
	entries, _ := b.History(0, 1, log.LevelDebug)
	if len(entries) != 1 {
		t.Fatalf("History length = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Name != "root.worker.sync" {
		t.Fatalf("Name = %q, want root.worker.sync", entry.Name)
	}
	if entry.Attrs["a"] != 1 || entry.Attrs["b"] != 2 || entry.Attrs["same"] != "entry" {
		t.Fatalf("Attrs = %#v, want accumulated attrs with entry override", entry.Attrs)
	}
}

func TestSubscribeBackfillContinuityAndCursorState(t *testing.T) {
	b := New(4)
	for i := range 5 {
		b.Log(context.Background(), log.LevelInfo, "before", log.Int("i", i))
	}

	sub := b.Subscribe(SubscribeOptions{StreamID: b.ID(), FromSeq: 2})
	defer sub.Cancel()
	if sub.Truncated || sub.Reset {
		t.Fatalf("subscription state = truncated:%v reset:%v, want neither", sub.Truncated, sub.Reset)
	}
	wantSeqs(t, sub.Backfill, 3, 4, 5)

	b.Log(context.Background(), log.LevelInfo, "live")
	select {
	case entry := <-sub.C:
		if entry.Seq != 6 {
			t.Fatalf("first live Seq = %d, want 6", entry.Seq)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live entry")
	}

	truncated := b.Subscribe(SubscribeOptions{StreamID: b.ID(), FromSeq: 1})
	defer truncated.Cancel()
	if !truncated.Truncated {
		t.Fatal("Truncated = false, want true for an overwritten cursor")
	}
	wantSeqs(t, truncated.Backfill, 3, 4, 5, 6)

	reset := b.Subscribe(SubscribeOptions{StreamID: "previous-process", FromSeq: 99, TailLimit: 2})
	defer reset.Cancel()
	if !reset.Reset || reset.Truncated {
		t.Fatalf("reset subscription state = truncated:%v reset:%v", reset.Truncated, reset.Reset)
	}
	wantSeqs(t, reset.Backfill, 5, 6)
}

// TestSubscribeCursorAtOrPastNewestDoesNotWrap pins the client-supplied cursor
// extremes: FromSeq is opaque input, and math.MaxUint64 must not wrap FromSeq+1
// into a false Truncated report that dumps the entire retained ring.
func TestSubscribeCursorAtOrPastNewestDoesNotWrap(t *testing.T) {
	b := New(4)
	for range 3 {
		b.Log(context.Background(), log.LevelInfo, "entry")
	}

	for _, from := range []uint64{b.LatestSeq(), math.MaxUint64} {
		sub := b.Subscribe(SubscribeOptions{StreamID: b.ID(), FromSeq: from})
		if sub.Truncated || len(sub.Backfill) != 0 {
			t.Fatalf("FromSeq %d: truncated=%v backfill=%d, want an empty backfill",
				from, sub.Truncated, len(sub.Backfill))
		}
		sub.Cancel()
	}
}

func TestSubscribeMinLevelAndDropped(t *testing.T) {
	b := New(8)
	b.Log(context.Background(), log.LevelInfo, "history info")
	b.Log(context.Background(), log.LevelError, "history error")

	sub := b.Subscribe(SubscribeOptions{
		TailLimit:     -1,
		MinLevel:      log.LevelWarn,
		ChannelBuffer: 1,
	})
	defer sub.Cancel()
	wantSeqs(t, sub.Backfill, 2)

	b.Log(context.Background(), log.LevelInfo, "live filtered")
	b.Log(context.Background(), log.LevelWarn, "live one")
	b.Log(context.Background(), log.LevelError, "live dropped")
	if got := sub.Dropped(); got != 1 {
		t.Fatalf("Dropped = %d, want 1", got)
	}
	if got := (<-sub.C).Seq; got != 4 {
		t.Fatalf("buffered live Seq = %d, want 4", got)
	}

	sub.Cancel()
	b.Log(context.Background(), log.LevelError, "after cancel")
	select {
	case entry := <-sub.C:
		t.Fatalf("received entry after Cancel: %#v", entry)
	default:
	}
}

func TestHistoryPagesBackwards(t *testing.T) {
	b := New(8)
	for i := range 6 {
		level := log.LevelDebug
		if i%2 == 1 {
			level = log.LevelInfo
		}
		b.Log(context.Background(), level, "entry")
	}

	entries, more := b.History(0, 2, log.LevelInfo)
	wantSeqs(t, entries, 4, 6)
	if !more {
		t.Fatal("first page more = false, want true")
	}

	entries, more = b.History(entries[0].Seq, 2, log.LevelInfo)
	wantSeqs(t, entries, 2)
	if more {
		t.Fatal("second page more = true, want false because no older matching entry remains")
	}
}

func TestConcurrentPublishPreservesSequenceOrder(t *testing.T) {
	const count = 128
	b := New(count)
	sub := b.Subscribe(SubscribeOptions{ChannelBuffer: count})
	defer sub.Cancel()

	var wg sync.WaitGroup
	for range count {
		wg.Go(func() {
			b.Log(context.Background(), log.LevelInfo, "entry")
		})
	}
	wg.Wait()

	for want := uint64(1); want <= count; want++ {
		if got := (<-sub.C).Seq; got != want {
			t.Fatalf("live sequence = %d, want %d", got, want)
		}
	}
}

type panickingLogValuer struct{}

func (panickingLogValuer) LogValue() slog.Value {
	panic("broken value")
}

func TestPanickingAttrDoesNotEscapeLog(t *testing.T) {
	b := New(1)
	b.Log(context.Background(), log.LevelError, "failure", log.Any("broken", panickingLogValuer{}))

	entries, _ := b.History(0, 1, log.LevelDebug)
	if len(entries) != 1 {
		t.Fatalf("History length = %d, want 1", len(entries))
	}
	if _, ok := entries[0].Attrs["broken"]; !ok {
		t.Fatalf("Attrs = %#v, want the safely resolved broken attr", entries[0].Attrs)
	}
}

func wantSeqs(t *testing.T, entries []Entry, want ...uint64) {
	t.Helper()
	if len(entries) != len(want) {
		t.Fatalf("entry count = %d, want %d: %#v", len(entries), len(want), entries)
	}
	for i, entry := range entries {
		if entry.Seq != want[i] {
			t.Fatalf("entry[%d].Seq = %d, want %d", i, entry.Seq, want[i])
		}
	}
}

// TestBufferStoresErrorAttrsAsMessages pins that an error attr serializes to its
// message: error values are usually structs with no exported fields, which
// encoding/json renders as {}.
func TestBufferStoresErrorAttrsAsMessages(t *testing.T) {
	var nilErr error
	for _, tc := range []struct {
		name string
		attr log.Attr
		want any
	}{
		{name: "error", attr: log.Err(errors.New("boom")), want: "boom"},
		{name: "wrapped error", attr: log.Err(fmt.Errorf("ctx: %w", errors.New("boom"))), want: "ctx: boom"},
		{name: "nil error", attr: log.Err(nilErr), want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := New(2)
			b.Log(context.Background(), log.LevelError, "failed", tc.attr)

			entries, _ := b.History(0, 1, log.LevelDebug)
			if len(entries) != 1 {
				t.Fatalf("entries = %d, want 1", len(entries))
			}
			if got := entries[0].Attrs["error"]; got != tc.want {
				t.Fatalf("error attr = %#v, want %#v", got, tc.want)
			}
			raw, err := json.Marshal(entries[0])
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if tc.want != nil && !strings.Contains(string(raw), fmt.Sprint(tc.want)) {
				t.Fatalf("json = %s, want it to contain %q", raw, tc.want)
			}
		})
	}
}

type panickingError struct{}

func (panickingError) Error() string { panic("broken error") }

type derefError struct{ msg string }

func (e *derefError) Error() string { return e.msg }

// TestBufferTypedNilErrorKeepsLaterAttrs pins that a typed nil error pointer is
// recorded as nil and that a panicking attr does not drop the attrs after it.
func TestBufferTypedNilErrorKeepsLaterAttrs(t *testing.T) {
	var typedNil *derefError
	b := New(2)
	b.Log(context.Background(), log.LevelError, "failed",
		log.Any("cause", typedNil),
		log.Any("broken", panickingError{}),
		log.String("request_id", "r1"))

	entries, _ := b.History(0, 1, log.LevelDebug)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	attrs := entries[0].Attrs
	if v, ok := attrs["cause"]; !ok || v != nil {
		t.Fatalf("cause = %#v (present %v), want nil", v, ok)
	}
	if attrs["request_id"] != "r1" {
		t.Fatalf("request_id = %#v, want it kept after earlier attrs: %#v", attrs["request_id"], attrs)
	}
	if _, ok := attrs["attr_error"]; !ok {
		t.Fatalf("attr_error missing for the panicking attr: %#v", attrs)
	}
}

// TestBufferPresetErrorAttrsAreMessages pins that error values in preset attrs,
// from New and from With, serialize to their message like per-call attrs.
func TestBufferPresetErrorAttrsAreMessages(t *testing.T) {
	b := New(2, log.WithAttrs(map[string]any{"cause": errors.New("boom")}))
	derived := b.With(log.WithAttrs(map[string]any{"other": fmt.Errorf("ctx: %w", errors.New("bang"))}))
	derived.Log(context.Background(), log.LevelError, "failed")

	entries, _ := b.History(0, 1, log.LevelDebug)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	raw, err := json.Marshal(entries[0].Attrs)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if want := `{"cause":"boom","other":"ctx: bang"}`; string(raw) != want {
		t.Fatalf("attrs JSON = %s, want %s", raw, want)
	}
}
