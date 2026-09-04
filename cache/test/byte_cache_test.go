package test

import (
	"bytes"
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

			ctx := t.Context()
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
			if len(got) != 2 || string(got["k2"]) != "v2" || string(got["k3"]) != "v3" {
				t.Fatalf("MultiGet mismatch: %#v", got)
			}
			if _, ok := got["k_missing"]; ok {
				t.Fatalf("MultiGet returned missing key: %#v", got)
			}

			if err := c.MultiDel(ctx, []string{"k2", "k3"}); err != nil {
				t.Fatalf("MultiDel: %v", err)
			}
			for _, key := range []string{"k2", "k3"} {
				exists, err = c.Exists(ctx, key)
				if err != nil {
					t.Fatalf("Exists %q after MultiDel: %v", key, err)
				}
				if exists {
					t.Fatalf("Exists %q after MultiDel: expected not found", key)
				}
			}

			if err := c.Del(ctx, "k4"); err != nil {
				t.Fatalf("Del: %v", err)
			}
			if exists, err = c.Exists(ctx, "k4"); err != nil || exists {
				t.Fatalf("Exists k4 after Del: exists=%v err=%v", exists, err)
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
			for _, key := range []string{"k5", "k6"} {
				exists, err = c.Exists(ctx, key)
				if err != nil {
					t.Fatalf("Exists %q after DelAll: %v", key, err)
				}
				if exists {
					t.Fatalf("Exists %q after DelAll: expected not found", key)
				}
			}
		})
	}
}

func TestByteCacheTTLContract(t *testing.T) {
	t.Parallel()

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			c := factory.new(t)

			if err := c.SetWithTTL(ctx, "ttl_key", []byte("ttl_value"), 250*time.Millisecond); err != nil {
				t.Fatalf("SetWithTTL: %v", err)
			}
			assertCacheValue(t, c, "ttl_key", "ttl_value")
			assertEventuallyNotFound(t, c, "ttl_key")

			if err := c.MultiSetWithTTL(ctx, map[string][]byte{
				"ttl_a": []byte("a"),
				"ttl_b": []byte("b"),
			}, 250*time.Millisecond); err != nil {
				t.Fatalf("MultiSetWithTTL: %v", err)
			}
			assertCacheValue(t, c, "ttl_a", "a")
			assertCacheValue(t, c, "ttl_b", "b")
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

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			c := factory.new(t)

			if err := c.Set(ctx, "neg_key", []byte("sentinel")); err != nil {
				t.Fatalf("seed neg_key: %v", err)
			}
			if err := c.SetWithTTL(ctx, "neg_key", []byte("replacement"), -time.Second); !errors.Is(err, cache.ErrInvalidTTL) {
				t.Fatalf("SetWithTTL negative: got err=%v want ErrInvalidTTL", err)
			}
			assertCacheValue(t, c, "neg_key", "sentinel")

			if err := c.Set(ctx, "neg_a", []byte("sentinel-a")); err != nil {
				t.Fatalf("seed neg_a: %v", err)
			}
			if err := c.MultiSetWithTTL(ctx, map[string][]byte{
				"neg_a": []byte("replacement-a"),
			}, -time.Second); !errors.Is(err, cache.ErrInvalidTTL) {
				t.Fatalf("MultiSetWithTTL negative: got err=%v want ErrInvalidTTL", err)
			}
			assertCacheValue(t, c, "neg_a", "sentinel-a")
		})
	}

	t.Run("nocache", func(t *testing.T) {
		c := noCacheFactory().new(t)
		if err := c.SetWithTTL(t.Context(), "neg", []byte("v"), -time.Second); !errors.Is(err, cache.ErrInvalidTTL) {
			t.Fatalf("SetWithTTL negative: got err=%v want ErrInvalidTTL", err)
		}
	})
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
			if !found || !bytes.Equal(got, large) {
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

func TestByteCacheGetDelExactlyOnce(t *testing.T) {
	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ctx := t.Context()
			c := factory.new(t)

			const keys = 32
			const consumers = 8

			for i := range keys {
				key := fmt.Sprintf("once_%d", i)
				if err := c.Set(ctx, key, []byte("token")); err != nil {
					t.Fatalf("Set: %v", err)
				}
				assertCacheValue(t, c, key, "token")
			}

			hits := make([]atomic.Int64, keys)
			errCh := make(chan error, keys*consumers)

			var wg sync.WaitGroup
			for i := range keys {
				for range consumers {
					wg.Go(func() {
						value, found, err := c.GetDel(ctx, fmt.Sprintf("once_%d", i))
						if err != nil {
							errCh <- err
							return
						}
						if found {
							if string(value) != "token" {
								errCh <- fmt.Errorf("unexpected value %q", value)
								return
							}
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

			for i := range keys {
				if got := hits[i].Load(); got != 1 {
					t.Fatalf("key once_%d consumed %d times, want exactly 1", i, got)
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

	ctx := t.Context()
	deadline := time.Now().Add(2 * time.Second)
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

func assertCacheValue(t *testing.T, c cache.ByteCache, key, want string) {
	t.Helper()
	value, found, err := c.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("Get %q: %v", key, err)
	}
	if !found || string(value) != want {
		t.Fatalf("Get %q: found=%v value=%q want=%q", key, found, value, want)
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
				stored := []byte("value")
				if err := c.Set(ctx, "own-get", stored); err != nil {
					t.Fatalf("Set: %v", err)
				}

				first, found, err := c.Get(ctx, "own-get")
				if err != nil || !found {
					t.Fatalf("Get: found=%v err=%v", found, err)
				}
				first[0] = 'X'

				second, found, err := c.Get(ctx, "own-get")
				if err != nil || !found {
					t.Fatalf("Get again: found=%v err=%v", found, err)
				}
				if string(second) != "value" {
					t.Fatalf("mutating a returned value changed the cached value: got %q", second)
				}
			})
		})
	}
}
