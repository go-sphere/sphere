package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/go-sphere/sphere/cache"
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

		if _, found, err := c.Get(ctx, "k"); !errors.Is(err, cache.ErrClosed) || found {
			t.Fatalf("owned cache must report ErrClosed after Close: found=%v err=%v", found, err)
		}
	})
}

// TestCloseConcurrentOperations pins that Close may race with any other method.
// task.Group cancels its members concurrently, so an in-flight request can still
// be writing while the cache is torn down. ristretto sets its own closed flag
// last, after closing setBuf and the stop channel, so reaching it during that
// window parks Set on Wait forever or panics with "send on closed channel".
func TestCloseConcurrentOperations(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		op   func(c *ByteCache) error
	}{
		{"Set", func(c *ByteCache) error { return c.Set(ctx, "k", []byte("v")) }},
		{"SetWithTTL", func(c *ByteCache) error { return c.SetWithTTL(ctx, "k", []byte("v"), time.Minute) }},
		{"Del", func(c *ByteCache) error { return c.Del(ctx, "k") }},
		{"DelAll", func(c *ByteCache) error { return c.DelAll(ctx) }},
		{"Get", func(c *ByteCache) error { _, _, err := c.Get(ctx, "k"); return err }},
		{"Sync", func(c *ByteCache) error { return c.Sync() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Each cache allocates defaultNumCounters counters, so the iteration
			// count is kept low enough to stay cheap under -race. The failure
			// modes this guards against (deadlock on Wait, send on closed
			// channel) reproduce within a handful of rounds.
			for i := 0; i < 25; i++ {
				c := NewByteCache()

				var wg sync.WaitGroup
				start := make(chan struct{})
				wg.Go(func() {
					<-start
					// Either outcome is valid; neither may deadlock or panic.
					if err := tc.op(c); err != nil && !errors.Is(err, cache.ErrClosed) {
						t.Errorf("unexpected error: %v", err)
					}
				})
				wg.Go(func() {
					<-start
					if err := c.Close(); err != nil {
						t.Errorf("Close: %v", err)
					}
				})
				close(start)
				wg.Wait()
			}
		})
	}
}

// TestCloseIsIdempotent pins that repeated Close calls stay safe, which the
// staged shutdown path relies on when a Stop is retried after a timeout.
func TestCloseIsIdempotent(t *testing.T) {
	c := NewByteCache()
	for i := 0; i < 3; i++ {
		if err := c.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i+1, err)
		}
	}
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

func TestGetDelIsAtomicWithSet(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache[string]()
	t.Cleanup(func() { _ = c.Close() })

	const attempts = 20_000
	for i := range attempts {
		if err := c.Set(ctx, "k", "old"); err != nil {
			t.Fatalf("seed attempt %d: %v", i, err)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		var got string
		var found bool
		wg.Go(func() {
			<-start
			if err := c.Set(ctx, "k", "new"); err != nil {
				t.Errorf("Set attempt %d: %v", i, err)
			}
		})
		wg.Go(func() {
			<-start
			var err error
			got, found, err = c.GetDel(ctx, "k")
			if err != nil {
				t.Errorf("GetDel attempt %d: %v", i, err)
			}
		})
		close(start)
		wg.Wait()

		final, exists, err := c.Get(ctx, "k")
		if err != nil {
			t.Fatalf("Get attempt %d: %v", i, err)
		}
		if found && got == "old" && !exists {
			t.Fatalf("non-linearizable result at attempt %d: GetDel returned old value and deleted concurrent Set", i)
		}
		if err := c.Del(ctx, "k"); err != nil {
			t.Fatalf("cleanup attempt %d (final=%q): %v", i, final, err)
		}
	}
}
