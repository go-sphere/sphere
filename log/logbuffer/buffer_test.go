package logbuffer

import (
	"context"
	"log/slog"
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
