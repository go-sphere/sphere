package test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestQueueContract(t *testing.T) {
	t.Parallel()

	for _, factory := range queueFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			q := factory.new(t)

			if err := q.Publish(ctx, "topic-1", 1); err != nil {
				t.Fatalf("Publish topic-1: %v", err)
			}
			msg, err := q.Consume(ctx, "topic-1")
			if err != nil {
				t.Fatalf("Consume topic-1: %v", err)
			}
			if msg != 1 {
				t.Fatalf("Consume topic-1 mismatch: got=%d want=1", msg)
			}

			if err := q.Publish(ctx, "topic-2", 2); err != nil {
				t.Fatalf("Publish topic-2 first: %v", err)
			}
			if err := q.Publish(ctx, "topic-2", 3); err != nil {
				t.Fatalf("Publish topic-2 second: %v", err)
			}
			first, err := q.Consume(ctx, "topic-2")
			if err != nil {
				t.Fatalf("Consume topic-2 first: %v", err)
			}
			second, err := q.Consume(ctx, "topic-2")
			if err != nil {
				t.Fatalf("Consume topic-2 second: %v", err)
			}
			if first != 2 || second != 3 {
				t.Fatalf("FIFO mismatch: first=%d second=%d", first, second)
			}

			_, found, err := q.TryConsume(ctx, "topic-2")
			if err != nil {
				t.Fatalf("TryConsume empty: %v", err)
			}
			if found {
				t.Fatalf("TryConsume empty mismatch: expected found=false")
			}
		})
	}
}

func TestQueuePurgeQueue(t *testing.T) {
	t.Parallel()

	for _, factory := range queueFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			q := factory.new(t)

			for i := range 3 {
				if err := q.Publish(ctx, "purge-topic", i); err != nil {
					t.Fatalf("Publish for purge: %v", err)
				}
			}

			if err := q.PurgeQueue(ctx, "purge-topic"); err != nil {
				t.Fatalf("PurgeQueue: %v", err)
			}

			_, found, err := q.TryConsume(ctx, "purge-topic")
			if err != nil {
				t.Fatalf("TryConsume after purge: %v", err)
			}
			if found {
				t.Fatalf("TryConsume after purge mismatch: expected empty queue")
			}

			if err := q.PurgeQueue(ctx, "missing-topic"); err != nil {
				t.Fatalf("PurgeQueue missing topic should be noop, got: %v", err)
			}
		})
	}
}

func TestQueueBlockingConsume(t *testing.T) {
	for _, factory := range queueFactories() {
		if !factory.blockingConsumeCheck {
			continue
		}

		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			q := factory.new(t)

			go func() {
				time.Sleep(40 * time.Millisecond)
				_ = q.Publish(context.Background(), "blocking-topic", 9)
			}()

			consumeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			msg, err := q.Consume(consumeCtx, "blocking-topic")
			if err != nil {
				t.Fatalf("blocking Consume: %v", err)
			}
			if msg != 9 {
				t.Fatalf("blocking Consume mismatch: got=%d want=9", msg)
			}

			timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer timeoutCancel()
			_, err = q.Consume(timeoutCtx, "no-message-topic")
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("empty blocking Consume mismatch: err=%v", err)
			}
		})
	}
}

func TestQueueTryConsumeCanceledContext(t *testing.T) {
	t.Parallel()

	for _, factory := range queueFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			q := factory.new(t)
			if err := q.Publish(context.Background(), "topic", 1); err != nil {
				t.Fatalf("seed queue for canceled TryConsume: %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, _, err := q.TryConsume(ctx, "topic")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("TryConsume canceled context mismatch: err=%v", err)
			}
		})
	}
}

func TestQueueClose(t *testing.T) {
	t.Parallel()

	for _, factory := range queueFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			q := factory.new(t)
			if err := q.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			_, _, err := q.TryConsume(context.Background(), "topic")
			if err == nil {
				t.Fatalf("expected error after Close")
			}
		})
	}
}

func TestQueueConcurrentPublishConsume(t *testing.T) {
	for _, factory := range queueFactories() {
		t.Run(factory.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("skip concurrent queue test in short mode")
			}

			ctx := context.Background()
			q := factory.new(t)

			const n = 64
			errCh := make(chan error, n)
			var wg sync.WaitGroup
			for i := range n {
				i := i
				wg.Go(func() {
					if err := q.Publish(ctx, "concurrent", i); err != nil {
						errCh <- err
					}
				})
			}
			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Fatalf("Publish concurrent: %v", err)
			}

			seen := make(map[int]bool, n)
			deadline := time.Now().Add(3 * time.Second)
			for len(seen) < n && time.Now().Before(deadline) {
				msg, found, err := q.TryConsume(ctx, "concurrent")
				if err != nil {
					t.Fatalf("TryConsume concurrent: %v", err)
				}
				if !found {
					time.Sleep(5 * time.Millisecond)
					continue
				}
				seen[msg] = true
			}

			if len(seen) != n {
				t.Fatalf("concurrent publish/consume lost messages: got=%d want=%d", len(seen), n)
			}
		})
	}
}
