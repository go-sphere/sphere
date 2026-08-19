package mcache

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestKeys(t *testing.T) {
	ctx := context.Background()
	c := NewByteCache()
	t.Cleanup(func() { _ = c.Close() })

	if err := c.MultiSet(ctx, map[string][]byte{
		"a:1": []byte("1"),
		"a:2": []byte("2"),
		"b:1": []byte("3"),
	}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}

	got, err := c.Keys(ctx, "a:")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	slices.Sort(got)
	want := []string{"a:1", "a:2"}
	if !slices.Equal(got, want) {
		t.Fatalf("Keys mismatch: got=%v want=%v", got, want)
	}

	all, err := c.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys empty prefix: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("Keys empty prefix len: got=%d want=3", len(all))
	}
}

func TestKeysSkipsExpired(t *testing.T) {
	ctx := context.Background()
	c := NewByteCache()
	t.Cleanup(func() { _ = c.Close() })

	if err := c.Set(ctx, "live", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.SetWithTTL(ctx, "expired", []byte("v"), time.Millisecond); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	keys, err := c.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if slices.Contains(keys, "expired") {
		t.Fatalf("Keys returned expired entry: %v", keys)
	}
	if !slices.Contains(keys, "live") {
		t.Fatalf("Keys missing live entry: %v", keys)
	}
}

func TestConcurrentExpiredReads(t *testing.T) {
	ctx := context.Background()
	c := NewByteCache()
	t.Cleanup(func() { _ = c.Close() })

	const keys = 32
	for i := range keys {
		key := string(rune('a' + i))
		if err := c.SetWithTTL(ctx, key, []byte("expired"), time.Millisecond); err != nil {
			t.Fatalf("SetWithTTL(%q): %v", key, err)
		}
	}
	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	for i := range keys {
		key := string(rune('a' + i))
		wg.Go(func() {
			for range 32 {
				if _, _, err := c.Get(ctx, key); err != nil {
					t.Errorf("Get(%q): %v", key, err)
					return
				}
				if _, err := c.MultiGet(ctx, []string{key}); err != nil {
					t.Errorf("MultiGet(%q): %v", key, err)
					return
				}
			}
		})
	}
	wg.Wait()
}

// TestCountAndTrimReclaimExpired covers the only bulk reclamation path this
// cache has. Expired entries stay resident in both maps until the key is
// touched again or Count/Trim sweeps, so the sweep is asserted against the
// internal maps rather than through Get.
func TestCountAndTrimReclaimExpired(t *testing.T) {
	ctx := context.Background()
	c := NewByteCache()

	if err := c.Set(ctx, "keep", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.SetWithTTL(ctx, "gone", []byte("v"), time.Millisecond); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	if len(c.store) != 2 {
		t.Fatalf("expired entry should still be resident before a sweep: store=%d", len(c.store))
	}

	if got := c.Count(); got != 1 {
		t.Fatalf("Count mismatch: got=%d want=1", got)
	}
	if len(c.store) != 1 || len(c.expiration) != 0 {
		t.Fatalf("Count must drop expired entries: store=%d expiration=%d", len(c.store), len(c.expiration))
	}

	if err := c.SetWithTTL(ctx, "gone2", []byte("v"), time.Millisecond); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	c.Trim()
	if len(c.store) != 1 || len(c.expiration) != 0 {
		t.Fatalf("Trim must drop expired entries: store=%d expiration=%d", len(c.store), len(c.expiration))
	}
}
