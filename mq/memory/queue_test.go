package memory

import (
	"context"
	"errors"
	"testing"
)

// TestQueueDeleteQueue verifies that DeleteQueue reclaims the internal map entry
// so the queues map does not grow without bound, and that a later Publish
// transparently recreates the topic.
func TestQueueDeleteQueue(t *testing.T) {
	ctx := context.Background()
	q := NewQueue[int]()
	t.Cleanup(func() { _ = q.Close() })

	if err := q.Publish(ctx, "topic", 1); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	q.mu.RLock()
	_, exists := q.queues["topic"]
	q.mu.RUnlock()
	if !exists {
		t.Fatalf("expected queues map to hold topic after Publish")
	}

	if err := q.DeleteQueue(ctx, "topic"); err != nil {
		t.Fatalf("DeleteQueue: %v", err)
	}

	q.mu.RLock()
	_, exists = q.queues["topic"]
	q.mu.RUnlock()
	if exists {
		t.Fatalf("expected queues map to no longer hold topic after DeleteQueue")
	}

	// A subsequent Publish recreates the queue transparently.
	if err := q.Publish(ctx, "topic", 2); err != nil {
		t.Fatalf("Publish after DeleteQueue: %v", err)
	}
	msg, found, err := q.TryConsume(ctx, "topic")
	if err != nil {
		t.Fatalf("TryConsume after recreate: %v", err)
	}
	if !found || msg != 2 {
		t.Fatalf("TryConsume after recreate mismatch: found=%v msg=%d", found, msg)
	}
}

// TestTryConsumeDoesNotCreateQueue pins that polling an unknown topic stays
// allocation-free. TryConsume never blocks, so it has no channel to wait on and
// no reason to register one that would then sit in the queues map forever —
// otherwise a consumer polling dynamically named topics grows the map without
// bound.
func TestTryConsumeDoesNotCreateQueue(t *testing.T) {
	ctx := context.Background()
	q := NewQueue[int]()
	t.Cleanup(func() { _ = q.Close() })

	msg, found, err := q.TryConsume(ctx, "unknown")
	if err != nil {
		t.Fatalf("TryConsume unknown topic: %v", err)
	}
	if found || msg != 0 {
		t.Fatalf("TryConsume unknown topic = (%d, %v), want (0, false)", msg, found)
	}

	q.mu.RLock()
	count := len(q.queues)
	q.mu.RUnlock()
	if count != 0 {
		t.Fatalf("queues map holds %d entries after TryConsume, want 0", count)
	}
}

// TestQueueDeleteQueueClosed verifies DeleteQueue reports the queue-closed error
// once the queue has been closed.
func TestQueueDeleteQueueClosed(t *testing.T) {
	q := NewQueue[int]()
	if err := q.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := q.DeleteQueue(context.Background(), "topic"); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("DeleteQueue after Close mismatch: err=%v", err)
	}
}

// TestQueueDeleteQueueCanceledContext keeps DeleteQueue aligned with the rest of
// the context-aware API: a caller that has already given up does not mutate the
// queue set.
func TestQueueDeleteQueueCanceledContext(t *testing.T) {
	q := NewQueue[int]()
	t.Cleanup(func() { _ = q.Close() })

	if err := q.Publish(context.Background(), "topic", 1); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := q.DeleteQueue(ctx, "topic"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DeleteQueue canceled context mismatch: err=%v", err)
	}

	q.mu.RLock()
	_, exists := q.queues["topic"]
	q.mu.RUnlock()
	if !exists {
		t.Fatalf("expected queues map to still hold topic after canceled DeleteQueue")
	}
}
