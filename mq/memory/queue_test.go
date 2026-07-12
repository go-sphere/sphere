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

	if err := q.DeleteQueue("topic"); err != nil {
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

// TestQueueDeleteQueueClosed verifies DeleteQueue reports the queue-closed error
// once the queue has been closed.
func TestQueueDeleteQueueClosed(t *testing.T) {
	q := NewQueue[int]()
	if err := q.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := q.DeleteQueue("topic"); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("DeleteQueue after Close mismatch: err=%v", err)
	}
}
