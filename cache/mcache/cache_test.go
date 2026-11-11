package mcache

import (
	"context"
	"fmt"
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
				value, found, err := c.Get(ctx, key)
				if err != nil {
					t.Errorf("Get(%q): %v", key, err)
					return
				}
				if found || value != nil {
					t.Errorf("Get(%q) returned expired value: found=%v value=%q", key, found, value)
					return
				}
				values, err := c.MultiGet(ctx, []string{key})
				if err != nil {
					t.Errorf("MultiGet(%q): %v", key, err)
					return
				}
				if _, ok := values[key]; ok {
					t.Errorf("MultiGet(%q) returned expired value", key)
					return
				}
			}
		})
	}
	wg.Wait()
}

// TestCountAndTrimReclaimExpired covers the bulk reclamation path this cache
// has. Expired entries stay resident in both maps until the key is touched
// again or Trim sweeps, so the sweep is asserted against the internal maps
// rather than through Get. Count is deliberately a cheap read: it may include
// expired-but-resident entries, so the sweep must come from Trim.
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

	// Count stays a read: it reports what is resident (including the expired
	// key) and does not reclaim.
	if got := c.Count(); got != 2 {
		t.Fatalf("Count mismatch: got=%d want=2", got)
	}
	if len(c.store) != 2 {
		t.Fatalf("Count must not drop expired entries: store=%d", len(c.store))
	}

	c.Trim()
	if len(c.store) != 1 || len(c.expiration) != 0 {
		t.Fatalf("Trim must drop expired entries: store=%d expiration=%d", len(c.store), len(c.expiration))
	}
}

// TestReadsShareReadLock pins that reads of unexpired entries need only the
// read lock: with another reader holding it, Get, MultiGet and Exists still
// complete instead of queueing behind each other.
func TestReadsShareReadLock(t *testing.T) {
	ctx := context.Background()
	c := NewByteCache()
	if err := c.Set(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}

	c.rw.RLock()
	done := make(chan error, 1)
	go func() {
		if _, found, err := c.Get(ctx, "k"); err != nil || !found {
			done <- fmt.Errorf("Get = (found=%v, err=%v)", found, err)
			return
		}
		if got, err := c.MultiGet(ctx, []string{"k"}); err != nil || len(got) != 1 {
			done <- fmt.Errorf("MultiGet = (%v, %v)", got, err)
			return
		}
		if found, err := c.Exists(ctx, "k"); err != nil || !found {
			done <- fmt.Errorf("Exists = (%v, %v)", found, err)
			return
		}
		done <- nil
	}()
	select {
	case err := <-done:
		c.rw.RUnlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		c.rw.RUnlock()
		t.Fatalf("reads blocked while another reader held the read lock (after release: %v)", <-done)
	}
}

// TestDeleteExpiredKeepsRefreshedEntry pins the re-check under the write
// lock: an entry a read saw as expired but that was rewritten before the
// delete ran is kept.
func TestDeleteExpiredKeepsRefreshedEntry(t *testing.T) {
	ctx := context.Background()
	c := NewByteCache()
	if err := c.SetWithTTL(ctx, "k", []byte("old"), time.Millisecond); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	// A reader has seen "k" expired; a writer refreshes it before the reader
	// takes the write lock.
	if err := c.SetWithTTL(ctx, "k", []byte("new"), time.Hour); err != nil {
		t.Fatalf("SetWithTTL refresh: %v", err)
	}
	c.deleteExpired([]string{"k"})

	got, found, err := c.Get(ctx, "k")
	if err != nil || !found || string(got) != "new" {
		t.Fatalf("Get = (%q, %v, %v), want (\"new\", true, nil)", got, found, err)
	}
}
