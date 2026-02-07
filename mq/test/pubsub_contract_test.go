package test

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPubSubContract(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			p := factory.newInt(t)

			const topic = "numbers"
			recv := make(chan int, 3)
			if err := p.Subscribe(ctx, topic, func(data int) error {
				recv <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}

			if err := p.Broadcast(ctx, topic, 1); err != nil {
				t.Fatalf("Broadcast first: %v", err)
			}
			if err := p.Broadcast(ctx, topic, 2); err != nil {
				t.Fatalf("Broadcast second: %v", err)
			}

			assertReceiveInt(t, recv, 1)
			assertReceiveInt(t, recv, 2)

			if err := p.UnsubscribeAll(ctx, topic); err != nil {
				t.Fatalf("UnsubscribeAll: %v", err)
			}

			if err := p.Broadcast(ctx, topic, 3); err != nil {
				t.Fatalf("Broadcast after unsubscribe should not fail: %v", err)
			}
			assertNoReceiveInt(t, recv)
		})
	}
}

func TestPubSubMultiSubscribers(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			p := factory.newInt(t)

			const topic = "fanout"
			recvA := make(chan int, 1)
			recvB := make(chan int, 1)

			if err := p.Subscribe(ctx, topic, func(data int) error {
				recvA <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe A: %v", err)
			}
			if err := p.Subscribe(ctx, topic, func(data int) error {
				recvB <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe B: %v", err)
			}

			if err := p.Broadcast(ctx, topic, 7); err != nil {
				t.Fatalf("Broadcast fanout: %v", err)
			}

			assertReceiveInt(t, recvA, 7)
			assertReceiveInt(t, recvB, 7)
		})
	}
}

func TestPubSubStructPayload(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			p := factory.newPayload(t)

			want := payload{
				ID:    42,
				Name:  "sphere",
				Meta:  map[string]string{"env": "test"},
				Flags: []bool{true, false, true},
			}

			recv := make(chan payload, 1)
			if err := p.Subscribe(ctx, "struct-topic", func(data payload) error {
				recv <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe struct payload: %v", err)
			}

			if err := p.Broadcast(ctx, "struct-topic", want); err != nil {
				t.Fatalf("Broadcast struct payload: %v", err)
			}

			got := assertReceivePayload(t, recv)
			if got.ID != want.ID || got.Name != want.Name || got.Meta["env"] != want.Meta["env"] {
				t.Fatalf("payload mismatch: got=%+v want=%+v", got, want)
			}
			if len(got.Flags) != len(want.Flags) {
				t.Fatalf("flags length mismatch: got=%d want=%d", len(got.Flags), len(want.Flags))
			}
			for i := range want.Flags {
				if got.Flags[i] != want.Flags[i] {
					t.Fatalf("flag mismatch at %d: got=%v want=%v", i, got.Flags[i], want.Flags[i])
				}
			}
		})
	}
}

func TestPubSubClose(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			p := factory.newInt(t)
			if err := p.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			if err := p.Close(); err != nil {
				t.Fatalf("Close idempotent check: %v", err)
			}
		})
	}
}

func TestPubSubConcurrentBroadcast(t *testing.T) {
	for _, factory := range pubSubFactories() {
		factory := factory
		t.Run(factory.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("skip concurrent pubsub test in short mode")
			}

			ctx := context.Background()
			p := factory.newInt(t)

			const n = 32
			const topic = "concurrent-topic"
			recv := make(chan int, n)
			errCh := make(chan error, n)

			if err := p.Subscribe(ctx, topic, func(data int) error {
				recv <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}

			var wg sync.WaitGroup
			for i := 0; i < n; i++ {
				i := i
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := p.Broadcast(ctx, topic, i); err != nil {
						errCh <- err
					}
				}()
			}
			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Fatalf("Broadcast concurrent: %v", err)
			}

			seen := make(map[int]bool, n)
			deadline := time.Now().Add(2 * time.Second)
			for len(seen) < n && time.Now().Before(deadline) {
				select {
				case v := <-recv:
					seen[v] = true
				case <-time.After(10 * time.Millisecond):
				}
			}

			if len(seen) != n {
				t.Fatalf("concurrent broadcast lost messages: got=%d want=%d", len(seen), n)
			}
		})
	}
}

func assertReceiveInt(t *testing.T, ch <-chan int, want int) {
	t.Helper()

	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("received value mismatch: got=%d want=%d", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for value=%d", want)
	}
}

func assertNoReceiveInt(t *testing.T, ch <-chan int) {
	t.Helper()

	select {
	case got := <-ch:
		t.Fatalf("unexpected message after unsubscribe: got=%d", got)
	case <-time.After(150 * time.Millisecond):
	}
}

func assertReceivePayload(t *testing.T, ch <-chan payload) payload {
	t.Helper()

	select {
	case got := <-ch:
		return got
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for payload")
		return payload{}
	}
}
