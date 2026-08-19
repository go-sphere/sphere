package test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/cache"
)

func TestByteCacheCoreContract(t *testing.T) {
	t.Parallel()

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := factory.new(t)

			if err := c.Set(ctx, "k1", []byte("v1")); err != nil {
				t.Fatalf("Set: %v", err)
			}
			v, found, err := c.Get(ctx, "k1")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if !found || string(v) != "v1" {
				t.Fatalf("Get mismatch: found=%v value=%q", found, string(v))
			}

			exists, err := c.Exists(ctx, "k1")
			if err != nil {
				t.Fatalf("Exists: %v", err)
			}
			if !exists {
				t.Fatalf("Exists mismatch: expected key present")
			}

			gotDel, found, err := c.GetDel(ctx, "k1")
			if err != nil {
				t.Fatalf("GetDel: %v", err)
			}
			if !found || string(gotDel) != "v1" {
				t.Fatalf("GetDel mismatch: found=%v value=%q", found, string(gotDel))
			}

			_, found, err = c.Get(ctx, "k1")
			if err != nil {
				t.Fatalf("Get after GetDel: %v", err)
			}
			if found {
				t.Fatalf("Get after GetDel mismatch: expected not found")
			}

			if err := c.MultiSet(ctx, map[string][]byte{
				"k2": []byte("v2"),
				"k3": []byte("v3"),
				"k4": []byte("v4"),
			}); err != nil {
				t.Fatalf("MultiSet: %v", err)
			}

			got, err := c.MultiGet(ctx, []string{"k2", "k3", "k_missing"})
			if err != nil {
				t.Fatalf("MultiGet: %v", err)
			}
			if string(got["k2"]) != "v2" || string(got["k3"]) != "v3" {
				t.Fatalf("MultiGet mismatch: %#v", got)
			}

			if err := c.MultiDel(ctx, []string{"k2", "k3"}); err != nil {
				t.Fatalf("MultiDel: %v", err)
			}
			exists, err = c.Exists(ctx, "k2")
			if err != nil {
				t.Fatalf("Exists after MultiDel: %v", err)
			}
			if exists {
				t.Fatalf("Exists after MultiDel mismatch: expected not found")
			}

			if err := c.Del(ctx, "k4"); err != nil {
				t.Fatalf("Del: %v", err)
			}

			if err := c.MultiSet(ctx, map[string][]byte{
				"k5": []byte("v5"),
				"k6": []byte("v6"),
			}); err != nil {
				t.Fatalf("MultiSet before DelAll: %v", err)
			}
			if err := c.DelAll(ctx); err != nil {
				t.Fatalf("DelAll: %v", err)
			}
			exists, err = c.Exists(ctx, "k6")
			if err != nil {
				t.Fatalf("Exists after DelAll: %v", err)
			}
			if exists {
				t.Fatalf("Exists after DelAll mismatch: expected not found")
			}
		})
	}
}

func TestByteCacheTTLContract(t *testing.T) {
	t.Parallel()

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := factory.new(t)

			if err := c.SetWithTTL(ctx, "ttl_key", []byte("ttl_value"), 20*time.Millisecond); err != nil {
				t.Fatalf("SetWithTTL: %v", err)
			}
			assertEventuallyNotFound(t, c, "ttl_key")

			if err := c.MultiSetWithTTL(ctx, map[string][]byte{
				"ttl_a": []byte("a"),
				"ttl_b": []byte("b"),
			}, 20*time.Millisecond); err != nil {
				t.Fatalf("MultiSetWithTTL: %v", err)
			}
			assertEventuallyNotFound(t, c, "ttl_a")
			assertEventuallyNotFound(t, c, "ttl_b")
		})
	}
}

func TestByteCacheTTLZeroNeverExpires(t *testing.T) {
	t.Parallel()

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := factory.new(t)

			// expiration == 0 means the entry never expires.
			if err := c.SetWithTTL(ctx, "zero_key", []byte("zero_value"), 0); err != nil {
				t.Fatalf("SetWithTTL zero: %v", err)
			}
			if err := c.MultiSetWithTTL(ctx, map[string][]byte{
				"zero_a": []byte("a"),
				"zero_b": []byte("b"),
			}, 0); err != nil {
				t.Fatalf("MultiSetWithTTL zero: %v", err)
			}

			time.Sleep(50 * time.Millisecond)

			for _, key := range []string{"zero_key", "zero_a", "zero_b"} {
				_, found, err := c.Get(ctx, key)
				if err != nil {
					t.Fatalf("Get %q: %v", key, err)
				}
				if !found {
					t.Fatalf("expected key %q to persist with zero TTL", key)
				}
			}
		})
	}
}

// Rejecting a negative TTL is about argument validation, not about storing
// anything, so nocache is included here even though it opts out of the
// persistence contracts above.
func TestByteCacheNegativeTTLRejected(t *testing.T) {
	t.Parallel()

	for _, factory := range append(statefulByteCacheFactories(), noCacheFactory()) {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := factory.new(t)

			if err := c.SetWithTTL(ctx, "neg_key", []byte("v"), -time.Second); !errors.Is(err, cache.ErrInvalidTTL) {
				t.Fatalf("SetWithTTL negative: got err=%v want ErrInvalidTTL", err)
			}
			exists, err := c.Exists(ctx, "neg_key")
			if err != nil {
				t.Fatalf("Exists after negative SetWithTTL: %v", err)
			}
			if exists {
				t.Fatalf("negative SetWithTTL must not store the key")
			}

			if err := c.MultiSetWithTTL(ctx, map[string][]byte{
				"neg_a": []byte("a"),
			}, -time.Second); !errors.Is(err, cache.ErrInvalidTTL) {
				t.Fatalf("MultiSetWithTTL negative: got err=%v want ErrInvalidTTL", err)
			}
			exists, err = c.Exists(ctx, "neg_a")
			if err != nil {
				t.Fatalf("Exists after negative MultiSetWithTTL: %v", err)
			}
			if exists {
				t.Fatalf("negative MultiSetWithTTL must not store the key")
			}
		})
	}
}

func TestByteCacheBoundaryContract(t *testing.T) {
	t.Parallel()

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := factory.new(t)

			if err := c.Set(ctx, "nil_value", nil); err != nil {
				t.Fatalf("Set nil value: %v", err)
			}
			v, found, err := c.Get(ctx, "nil_value")
			if err != nil {
				t.Fatalf("Get nil value: %v", err)
			}
			if !found || len(v) != 0 {
				t.Fatalf("nil value mismatch: found=%v len=%d", found, len(v))
			}

			large := make([]byte, 1<<20)
			for i := range large {
				large[i] = byte(i % 251)
			}
			if err := c.Set(ctx, "large", large); err != nil {
				t.Fatalf("Set large value: %v", err)
			}
			got, found, err := c.Get(ctx, "large")
			if err != nil {
				t.Fatalf("Get large value: %v", err)
			}
			if !found || len(got) != len(large) {
				t.Fatalf("large value mismatch: found=%v len=%d", found, len(got))
			}
		})
	}
}

func TestNoCacheContract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := noCacheFactory().new(t)

	if err := c.Set(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	v, found, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found || len(v) != 0 {
		t.Fatalf("NoCache Get mismatch: found=%v len=%d", found, len(v))
	}

	if err := c.MultiSet(ctx, map[string][]byte{"a": []byte("1"), "b": []byte("2")}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}
	got, err := c.MultiGet(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatalf("MultiGet: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("NoCache MultiGet mismatch: %#v", got)
	}

	exists, err := c.Exists(ctx, "a")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Fatalf("NoCache Exists mismatch: expected false")
	}
}

func TestByteCacheConcurrentAccess(t *testing.T) {
	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("skip concurrent test in short mode")
			}

			ctx := context.Background()
			c := factory.new(t)

			const writers = 8
			const perWriter = 64

			var wg sync.WaitGroup
			errCh := make(chan error, writers)

			for i := range writers {
				wg.Go(func() {
					for j := range perWriter {
						key := fmt.Sprintf("k_%d_%d", i, j)
						val := fmt.Appendf(nil, "v_%d_%d", i, j)
						if err := c.Set(ctx, key, val); err != nil {
							errCh <- err
							return
						}
					}
				})
			}

			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Fatalf("concurrent Set: %v", err)
			}

			for i := range writers {
				for j := range perWriter {
					key := fmt.Sprintf("k_%d_%d", i, j)
					want := fmt.Sprintf("v_%d_%d", i, j)
					val, found, err := c.Get(ctx, key)
					if err != nil {
						t.Fatalf("Get %s: %v", key, err)
					}
					if !found || string(val) != want {
						t.Fatalf("Get %s mismatch: found=%v value=%q want=%q", key, found, string(val), want)
					}
				}
			}
		})
	}
}

func TestByteCacheGetDelAtMostOnce(t *testing.T) {
	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("skip concurrent test in short mode")
			}

			ctx := context.Background()
			c := factory.new(t)

			const keys = 200
			const consumers = 4

			for i := range keys {
				if err := c.Set(ctx, fmt.Sprintf("once_%d", i), []byte("token")); err != nil {
					t.Fatalf("Set: %v", err)
				}
			}

			hits := make([]atomic.Int64, keys)
			errCh := make(chan error, keys*consumers)

			var wg sync.WaitGroup
			for i := range keys {
				for range consumers {
					wg.Go(func() {
						_, found, err := c.GetDel(ctx, fmt.Sprintf("once_%d", i))
						if err != nil {
							errCh <- err
							return
						}
						if found {
							hits[i].Add(1)
						}
					})
				}
			}

			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Fatalf("concurrent GetDel: %v", err)
			}

			// Returning not-found is allowed (the memory driver may drop a
			// write under pressure), but an entry must never be handed to two
			// callers: GetDel backs one-shot tokens.
			for i := range keys {
				if got := hits[i].Load(); got > 1 {
					t.Fatalf("key once_%d consumed %d times, want at most 1", i, got)
				}
			}
		})
	}
}

func TestByteCacheClose(t *testing.T) {
	t.Parallel()

	for _, factory := range append(statefulByteCacheFactories(), noCacheFactory()) {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			c := factory.new(t)
			if err := c.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func assertEventuallyNotFound(t *testing.T, c cache.ByteCache, key string) {
	t.Helper()

	ctx := context.Background()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, found, err := c.Get(ctx, key)
		if err == nil && !found {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	_, found, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get for TTL check: %v", err)
	}
	if found {
		t.Fatalf("expected key %q to expire", key)
	}
}

// TestByteCacheValueOwnership pins that a byte cache holds copies, not the
// caller's backing array, in both directions.
//
// The in-process drivers used to store and return the caller's own slice, while
// redis and badgerdb always produce fresh ones. That split made the same
// application code correct on one backend and silently corrupting on another:
// reusing an encoding buffer after Set rewrote the cached entry, and appending
// to a value returned by Get wrote into it in place whenever the slice had spare
// capacity — which json.Marshal results routinely do.
func TestByteCacheValueOwnership(t *testing.T) {
	t.Parallel()

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := factory.new(t)

			t.Run("set does not alias the caller's buffer", func(t *testing.T) {
				buf := []byte("original")
				if err := c.Set(ctx, "own-set", buf); err != nil {
					t.Fatalf("Set: %v", err)
				}
				copy(buf, "MUTATED!")

				got, found, err := c.Get(ctx, "own-set")
				if err != nil || !found {
					t.Fatalf("Get: found=%v err=%v", found, err)
				}
				if string(got) != "original" {
					t.Fatalf("mutating the caller's buffer changed the cached value: got %q", got)
				}
			})

			t.Run("get does not alias the stored value", func(t *testing.T) {
				// Spare capacity is what makes an append write in place rather
				// than allocate, so the returned slice is grown deliberately.
				stored := make([]byte, 5, 32)
				copy(stored, "value")
				if err := c.Set(ctx, "own-get", stored); err != nil {
					t.Fatalf("Set: %v", err)
				}

				first, found, err := c.Get(ctx, "own-get")
				if err != nil || !found {
					t.Fatalf("Get: found=%v err=%v", found, err)
				}
				_ = append(first, "-appended"...)

				second, found, err := c.Get(ctx, "own-get")
				if err != nil || !found {
					t.Fatalf("Get again: found=%v err=%v", found, err)
				}
				if string(second) != "value" {
					t.Fatalf("appending to a returned value changed the cached value: got %q", second)
				}
			})
		})
	}
}
