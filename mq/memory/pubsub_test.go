package memory

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestBroadcastDropsWhenSubscriberQueueFull pins the back pressure contract: a
// stalled subscriber loses messages instead of blocking the whole Broadcast,
// which under a background context would otherwise hang forever.
func TestBroadcastDropsWhenSubscriberQueueFull(t *testing.T) {
	ctx := context.Background()
	p := NewPubSub[int](WithQueueSize(1))
	t.Cleanup(func() { _ = p.Stop(context.Background()) })

	started := make(chan struct{})
	release := make(chan struct{})
	received := make(chan int, 4)
	if _, err := p.Subscribe(ctx, "topic", func(_ context.Context, data int) error {
		if data == 1 {
			close(started)
			<-release
		}
		received <- data
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Park the handler on message 1, so the single buffer slot is the only room
	// left for the messages that follow.
	if err := p.Broadcast(ctx, "topic", 1); err != nil {
		t.Fatalf("Broadcast 1: %v", err)
	}
	<-started

	// 2 takes that slot; 3 has nowhere to go and is dropped rather than blocking.
	if err := p.Broadcast(ctx, "topic", 2); err != nil {
		t.Fatalf("Broadcast 2: %v", err)
	}
	if err := p.Broadcast(ctx, "topic", 3); err != nil {
		t.Fatalf("Broadcast 3 should neither block nor fail: %v", err)
	}

	close(release)
	for _, want := range []int{1, 2} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("received %d, want %d", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %d", want)
		}
	}
	select {
	case got := <-received:
		t.Fatalf("received %d, want the message broadcast into a full queue to be dropped", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestBroadcastCanceledContext pins that a canceled context is reported once, up
// front. Delivery is a non-blocking send, so a ctx case inside that select would
// only compete at random with a send that could have succeeded.
func TestBroadcastCanceledContext(t *testing.T) {
	p := NewPubSub[int]()
	t.Cleanup(func() { _ = p.Stop(context.Background()) })

	received := make(chan int, 1)
	if _, err := p.Subscribe(context.Background(), "topic", func(_ context.Context, data int) error {
		received <- data
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := p.Broadcast(ctx, "topic", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("Broadcast with canceled context = %v, want %v", err, context.Canceled)
	}
	select {
	case data := <-received:
		t.Fatalf("Broadcast delivered %d despite a canceled context", data)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestPubSubIgnoresInvalidQueueSize pins that a bad size falls back to the
// default rather than crippling delivery.
//
// A zero size produced an unbuffered channel, and because Broadcast sends
// non-blockingly, delivery then depended on a subscriber happening to be parked
// in its receive at that exact instant — an idle, perfectly healthy subscriber
// dropped most of its messages, while drop-on-full is meant to shed load only
// for a subscriber that has fallen behind. A negative size panicked in make,
// far from the configuration that caused it.
func TestPubSubIgnoresInvalidQueueSize(t *testing.T) {
	for _, size := range []int{0, -1} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			ps := NewPubSub[int](WithQueueSize(size))
			t.Cleanup(func() { _ = ps.Stop(context.Background()) })

			const total = 10
			received := make(chan int, total)
			if _, err := ps.Subscribe(context.Background(), "topic", func(_ context.Context, v int) error {
				received <- v
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}

			// Let the subscriber goroutine reach its receive loop.
			time.Sleep(50 * time.Millisecond)

			for i := range total {
				if err := ps.Broadcast(context.Background(), "topic", i); err != nil {
					t.Fatalf("Broadcast: %v", err)
				}
			}

			deadline := time.After(2 * time.Second)
			for got := 0; got < total; got++ {
				select {
				case <-received:
				case <-deadline:
					t.Fatalf("an idle subscriber received only %d of %d broadcasts", got, total)
				}
			}
		})
	}
}
