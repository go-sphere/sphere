package memory

import (
	"context"
	"errors"
	"testing"
	"time"
)

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

// TestConsumeRacingCloseNeverDropsMessages stresses Consume against a concurrent
// Close. It cannot force the specific interleaving where a buffered message and
// the shutdown signal become ready together (the runtime hands a published value
// straight to a parked receiver), so it is a smoke test for the race detector
// rather than a pin for the drain preference — the deterministic guarantee is
// covered by TestConsumeDrainsBufferedMessagesAfterClose.
func TestConsumeRacingCloseNeverDropsMessages(t *testing.T) {
	for range 200 {
		ctx := context.Background()
		q := NewQueue[int]()

		result := make(chan int, 1)
		errCh := make(chan error, 1)
		go func() {
			data, err := q.Consume(ctx, "topic")
			if err != nil {
				errCh <- err
				return
			}
			result <- data
		}()

		time.Sleep(time.Millisecond)
		_ = q.Publish(ctx, "topic", 7)
		_ = q.Close()

		select {
		case data := <-result:
			if data != 7 {
				t.Fatalf("Consume returned %d, want 7", data)
			}
		case err := <-errCh:
			if !errors.Is(err, ErrQueueClosed) {
				t.Fatalf("unexpected Consume error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Consume did not return")
		}
	}
}
