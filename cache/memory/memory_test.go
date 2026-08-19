package memory

import (
	"context"
	"sync"
	"testing"

	"github.com/dgraph-io/ristretto/v2"
)

// TestCloseOwnership pins the ownership rule: Close releases only a ristretto
// cache this Cache created. Getting it wrong is silent in both directions —
// a leak, or a shared cache closed out from under its owner, which ristretto
// then reports as an ordinary miss rather than an error.
func TestCloseOwnership(t *testing.T) {
	ctx := context.Background()

	t.Run("injected", func(t *testing.T) {
		inner, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
			NumCounters: 100,
			MaxCost:     1000,
			BufferItems: 64,
		})
		if err != nil {
			t.Fatalf("new ristretto: %v", err)
		}
		t.Cleanup(inner.Close)

		c := NewMemoryCacheWithRistretto(inner, false, false)
		if err := c.Set(ctx, "k", []byte("v")); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := c.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		if _, found := inner.Get("k"); !found {
			t.Fatalf("injected ristretto cache must stay open after Close")
		}
	})

	t.Run("owned", func(t *testing.T) {
		c := NewByteCache()
		if err := c.Set(ctx, "k", []byte("v")); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if _, found, err := c.Get(ctx, "k"); err != nil || !found {
			t.Fatalf("seed not stored: found=%v err=%v", found, err)
		}
		if err := c.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		if _, found, err := c.Get(ctx, "k"); err != nil || found {
			t.Fatalf("owned ristretto cache must be closed: found=%v err=%v", found, err)
		}
	})
}

// TestSetAllowAsyncWritesConcurrent guards allowAsyncWrites against being
// turned back into a plain field: Set reads it on every call while
// SetAllowAsyncWrites writes it. The failure is a data race, so this only
// reports under -race (see the verify target).
func TestSetAllowAsyncWritesConcurrent(t *testing.T) {
	ctx := context.Background()
	c := NewByteCache()
	t.Cleanup(func() { _ = c.Close() })

	const iterations = 2000

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	wg.Go(func() {
		for range iterations {
			if err := c.Set(ctx, "k", []byte("v")); err != nil {
				errCh <- err
				return
			}
		}
	})
	wg.Go(func() {
		for i := range iterations {
			c.SetAllowAsyncWrites(i%2 == 0)
		}
	})

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent Set: %v", err)
	}
}
