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

// TestConsumeDrainsBufferedMessagesAfterClose pins that Close is a shutdown
// signal rather than a discard: messages the queue already accepted stay
// deliverable, and ErrQueueClosed is only reported once the topic is empty.
func TestConsumeDrainsBufferedMessagesAfterClose(t *testing.T) {
	ctx := context.Background()
	q := NewQueue[int]()

	for i := range 3 {
		if err := q.Publish(ctx, "topic", i); err != nil {
			t.Fatalf("Publish(%d): %v", i, err)
		}
	}
	if err := q.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for want := range 3 {
		got, err := q.Consume(ctx, "topic")
		if err != nil {
			t.Fatalf("Consume after Close: %v", err)
		}
		if got != want {
			t.Fatalf("Consume returned %d, want %d", got, want)
		}
	}

	if _, err := q.Consume(ctx, "topic"); !errors.Is(err, ErrQueueClosed) {
		t.Errorf("drained queue error = %v, want %v", err, ErrQueueClosed)
	}
	if _, err := q.Consume(ctx, "unknown"); !errors.Is(err, ErrQueueClosed) {
		t.Errorf("unknown topic error = %v, want %v", err, ErrQueueClosed)
	}
}

// TestTryConsumeDrainsBufferedMessagesAfterClose keeps TryConsume aligned with
// Consume so the two do not disagree about what Close means.
func TestTryConsumeDrainsBufferedMessagesAfterClose(t *testing.T) {
	ctx := context.Background()
	q := NewQueue[int]()

	if err := q.Publish(ctx, "topic", 42); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, found, err := q.TryConsume(ctx, "topic")
	if err != nil {
		t.Fatalf("TryConsume after Close: %v", err)
	}
	if !found || got != 42 {
		t.Fatalf("TryConsume = (%d, %v), want (42, true)", got, found)
	}
	if _, _, err := q.TryConsume(ctx, "topic"); !errors.Is(err, ErrQueueClosed) {
		t.Errorf("drained queue error = %v, want %v", err, ErrQueueClosed)
	}
}

// TestPublishRacingClosePreservesAcceptedMessages checks the observable race
// contract: a message acknowledged by Publish remains consumable after Close.
func TestPublishRacingClosePreservesAcceptedMessages(t *testing.T) {
	for range 32 {
		ctx := t.Context()
		q := NewQueue[int]()

		start := make(chan struct{})
		publishResult := make(chan error, 1)
		closeResult := make(chan error, 1)
		go func() {
			<-start
			publishResult <- q.Publish(ctx, "topic", 7)
		}()
		go func() {
			<-start
			closeResult <- q.Close()
		}()
		close(start)

		publishErr := <-publishResult
		if err := <-closeResult; err != nil {
			t.Fatalf("Close: %v", err)
		}
		if errors.Is(publishErr, ErrQueueClosed) {
			continue
		}
		if publishErr != nil {
			t.Fatalf("Publish: %v", publishErr)
		}
		data, err := q.Consume(ctx, "topic")
		if err != nil || data != 7 {
			t.Fatalf("Consume accepted message = (%d, %v), want (7, nil)", data, err)
		}
	}
}
