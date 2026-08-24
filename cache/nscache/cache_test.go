package nscache_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/cache/nocache"
	"github.com/go-sphere/sphere/cache/nscache"
	"github.com/go-sphere/sphere/cache/redis"
	"github.com/go-sphere/sphere/test/redistest"
)

type backendFactory struct {
	name            string
	new             func(tb testing.TB) cache.ByteCache
	supportsListing bool
	// emptyKeyspace marks a backend that implements KeyLister but never stores
	// anything, so DelAll is supported while round-trip assertions are not.
	emptyKeyspace bool
}

func backendFactories() []backendFactory {
	return []backendFactory{
		{
			name: "mcache",
			new: func(tb testing.TB) cache.ByteCache {
				tb.Helper()
				c := mcache.NewByteCache()
				tb.Cleanup(func() { _ = c.Close() })
				return c
			},
			supportsListing: true,
		},
		{
			name: "badgerdb",
			new: func(tb testing.TB) cache.ByteCache {
				tb.Helper()
				c, err := badgerdb.NewDatabase(badgerdb.Config{Path: tb.TempDir()})
				if err != nil {
					tb.Fatalf("create badgerdb: %v", err)
				}
				tb.Cleanup(func() { _ = c.Close() })
				return c
			},
			supportsListing: true,
		},
		{
			name: "redis",
			new: func(tb testing.TB) cache.ByteCache {
				t, ok := tb.(*testing.T)
				if !ok {
					tb.Fatalf("redis factory requires *testing.T")
				}
				t.Helper()
				client := redistest.NewTestRedisClient(t)
				c := redis.NewByteCache(client)
				tb.Cleanup(func() { _ = c.Close() })
				return c
			},
			supportsListing: true,
		},
		{
			name: "memory",
			new: func(tb testing.TB) cache.ByteCache {
				tb.Helper()
				c := memory.NewByteCache()
				tb.Cleanup(func() { _ = c.Close() })
				return c
			},
			supportsListing: false,
		},
		{
			name: "nocache",
			new: func(tb testing.TB) cache.ByteCache {
				tb.Helper()
				c := nocache.NewByteNoCache()
				tb.Cleanup(func() { _ = c.Close() })
				return c
			},
			// nocache implements KeyLister — over an always-empty keyspace — so
			// DelAll succeeds, but it stores nothing, so it cannot take part in
			// the tests that read back what they wrote.
			supportsListing: false,
			emptyKeyspace:   true,
		},
	}
}

func TestNSCacheDelAllNamespaceIsolation(t *testing.T) {
	for _, factory := range backendFactories() {
		if !factory.supportsListing {
			continue
		}
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			backend := factory.new(t)

			ns1 := nscache.NewNSCache[[]byte]("ns1", backend)
			ns2 := nscache.NewNSCache[[]byte]("ns2", backend)

			for i := range 3 {
				key := fmt.Sprintf("k%d", i)
				if err := ns1.Set(ctx, key, []byte("v1")); err != nil {
					t.Fatalf("ns1.Set: %v", err)
				}
				if err := ns2.Set(ctx, key, []byte("v2")); err != nil {
					t.Fatalf("ns2.Set: %v", err)
				}
			}

			if err := ns1.DelAll(ctx); err != nil {
				t.Fatalf("ns1.DelAll: %v", err)
			}

			for i := range 3 {
				key := fmt.Sprintf("k%d", i)
				gone, err := ns1.Exists(ctx, key)
				if err != nil {
					t.Fatalf("ns1.Exists: %v", err)
				}
				if gone {
					t.Fatalf("ns1.%s should be deleted by DelAll", key)
				}
				kept, err := ns2.Exists(ctx, key)
				if err != nil {
					t.Fatalf("ns2.Exists: %v", err)
				}
				if !kept {
					t.Fatalf("ns2.%s should survive ns1.DelAll", key)
				}
				v, found, err := ns2.Get(ctx, key)
				if err != nil {
					t.Fatalf("ns2.Get: %v", err)
				}
				if !found || string(v) != "v2" {
					t.Fatalf("ns2.%s mismatch: found=%v val=%q", key, found, string(v))
				}
			}
		})
	}
}

func TestNewNSCacheCheckedValidatesNamespace(t *testing.T) {
	backend := mcache.NewByteCache()
	t.Cleanup(func() { _ = backend.Close() })

	for _, namespace := range []string{"a:b", ":"} {
		t.Run(namespace, func(t *testing.T) {
			got, err := nscache.NewNSCacheChecked[[]byte](namespace, backend)
			if !errors.Is(err, nscache.ErrInvalidNamespace) {
				t.Fatalf("NewNSCacheChecked(%q) error = %v, want ErrInvalidNamespace", namespace, err)
			}
			if got != nil {
				t.Fatalf("NewNSCacheChecked(%q) = %v, want nil", namespace, got)
			}
		})
	}

	got, err := nscache.NewNSCacheChecked[[]byte]("safe", backend)
	if err != nil {
		t.Fatalf("NewNSCacheChecked(valid): %v", err)
	}
	if got == nil {
		t.Fatal("NewNSCacheChecked(valid) returned nil")
	}
}

func TestNSCacheDelAllEmpty(t *testing.T) {
	for _, factory := range backendFactories() {
		if !factory.supportsListing {
			continue
		}
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			backend := factory.new(t)
			ns := nscache.NewNSCache[[]byte]("empty", backend)
			if err := ns.DelAll(ctx); err != nil {
				t.Fatalf("DelAll on empty namespace: %v", err)
			}
		})
	}
}

func TestNSCacheDelAllUnsupportedBackend(t *testing.T) {
	for _, factory := range backendFactories() {
		if factory.supportsListing || factory.emptyKeyspace {
			continue
		}
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			backend := factory.new(t)
			ns := nscache.NewNSCache[[]byte]("ns", backend)
			err := ns.DelAll(ctx)
			if !errors.Is(err, cache.ErrNotSupported) {
				t.Fatalf("expected cache.ErrNotSupported, got %v", err)
			}
		})
	}
}

// TestNSCacheDelAllThroughCodecCache covers the common production layering of
// NSCache(CodecCache(ByteCache)): DelAll must still scope deletion to the
// namespace instead of degrading to ErrNotSupported just because the
// immediate wrapped cache is a typed adapter.
func TestNSCacheDelAllThroughCodecCache(t *testing.T) {
	for _, factory := range backendFactories() {
		if !factory.supportsListing {
			continue
		}
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			backend := factory.new(t)
			typed := cache.NewJsonCache[string](backend)

			ns1 := nscache.NewNSCache[string]("ns1", typed)
			ns2 := nscache.NewNSCache[string]("ns2", typed)

			for i := range 3 {
				key := fmt.Sprintf("k%d", i)
				if err := ns1.Set(ctx, key, "v1"); err != nil {
					t.Fatalf("ns1.Set: %v", err)
				}
				if err := ns2.Set(ctx, key, "v2"); err != nil {
					t.Fatalf("ns2.Set: %v", err)
				}
			}

			if err := ns1.DelAll(ctx); err != nil {
				t.Fatalf("ns1.DelAll: %v", err)
			}

			for i := range 3 {
				key := fmt.Sprintf("k%d", i)
				if found, err := ns1.Exists(ctx, key); err != nil {
					t.Fatalf("ns1.Exists: %v", err)
				} else if found {
					t.Fatalf("ns1.%s should be deleted", key)
				}
				v, found, err := ns2.Get(ctx, key)
				if err != nil {
					t.Fatalf("ns2.Get: %v", err)
				}
				if !found || v != "v2" {
					t.Fatalf("ns2.%s mismatch: found=%v val=%q", key, found, v)
				}
			}
		})
	}
}

// TestNSCacheKeysStripsNamespace pins the key space Keys reports in: callers
// get the same unprefixed keys they passed to Set, so a result can be fed
// straight back into Get/MultiDel without knowing the namespace.
func TestNSCacheKeysStripsNamespace(t *testing.T) {
	for _, factory := range backendFactories() {
		if !factory.supportsListing {
			continue
		}
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			backend := factory.new(t)

			ns := nscache.NewNSCache[[]byte]("ns", backend)
			other := nscache.NewNSCache[[]byte]("other", backend)

			if err := ns.MultiSet(ctx, map[string][]byte{
				"user:1":  []byte("a"),
				"user:2":  []byte("b"),
				"order:1": []byte("c"),
			}); err != nil {
				t.Fatalf("MultiSet: %v", err)
			}
			if err := other.Set(ctx, "user:3", []byte("d")); err != nil {
				t.Fatalf("other.Set: %v", err)
			}

			keys, err := ns.Keys(ctx, "user:")
			if err != nil {
				t.Fatalf("Keys: %v", err)
			}
			got := make(map[string]bool, len(keys))
			for _, k := range keys {
				got[k] = true
			}
			if len(got) != 2 || !got["user:1"] || !got["user:2"] {
				t.Fatalf("Keys mismatch: %v", keys)
			}

			// The returned keys must be usable as-is against the same cache.
			if err := ns.MultiDel(ctx, keys); err != nil {
				t.Fatalf("MultiDel with returned keys: %v", err)
			}
			for _, k := range []string{"user:1", "user:2"} {
				if found, err := ns.Exists(ctx, k); err != nil {
					t.Fatalf("Exists %s: %v", k, err)
				} else if found {
					t.Fatalf("%s should be deleted", k)
				}
			}
			if found, err := ns.Exists(ctx, "order:1"); err != nil {
				t.Fatalf("Exists order:1: %v", err)
			} else if !found {
				t.Fatalf("order:1 must not match the user: prefix")
			}
			if found, err := other.Exists(ctx, "user:3"); err != nil {
				t.Fatalf("other.Exists: %v", err)
			} else if !found {
				t.Fatalf("sibling namespace must be untouched")
			}
		})
	}
}

// TestNSCacheNestedDelAll covers NSCache(NSCache(backend)). Before NSCache
// implemented cache.KeyLister the outer DelAll degraded to ErrNotSupported,
// because the cache it wraps is itself a wrapper.
func TestNSCacheNestedDelAll(t *testing.T) {
	for _, factory := range backendFactories() {
		if !factory.supportsListing {
			continue
		}
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			backend := factory.new(t)

			outer := nscache.NewNSCache[[]byte]("outer", backend)
			inner := nscache.NewNSCache[[]byte]("inner", outer)
			sibling := nscache.NewNSCache[[]byte]("sibling", outer)

			if err := inner.Set(ctx, "k", []byte("v")); err != nil {
				t.Fatalf("inner.Set: %v", err)
			}
			if err := sibling.Set(ctx, "k", []byte("v")); err != nil {
				t.Fatalf("sibling.Set: %v", err)
			}

			if err := inner.DelAll(ctx); err != nil {
				t.Fatalf("inner.DelAll: %v", err)
			}

			if found, err := inner.Exists(ctx, "k"); err != nil {
				t.Fatalf("inner.Exists: %v", err)
			} else if found {
				t.Fatalf("inner key should be deleted")
			}
			if found, err := sibling.Exists(ctx, "k"); err != nil {
				t.Fatalf("sibling.Exists: %v", err)
			} else if !found {
				t.Fatalf("sibling key should survive inner.DelAll")
			}
		})
	}
}

// TestNSCacheKeyMapping covers every method that rewrites keys. The backend is
// inspected directly so the test pins the stored key layout rather than only
// the wrapper's self-consistency.
func TestNSCacheKeyMapping(t *testing.T) {
	for _, factory := range backendFactories() {
		if !factory.supportsListing {
			continue
		}
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			backend := factory.new(t)
			ns := nscache.NewNSCache[[]byte]("ns", backend)

			if err := ns.MultiSet(ctx, map[string][]byte{"a": []byte("1"), "b": []byte("2")}); err != nil {
				t.Fatalf("MultiSet: %v", err)
			}
			if _, found, err := backend.Get(ctx, "ns:a"); err != nil {
				t.Fatalf("backend.Get: %v", err)
			} else if !found {
				t.Fatalf("MultiSet must store under the namespace prefix")
			}

			got, err := ns.MultiGet(ctx, []string{"a", "b", "absent"})
			if err != nil {
				t.Fatalf("MultiGet: %v", err)
			}
			if len(got) != 2 || string(got["a"]) != "1" || string(got["b"]) != "2" {
				t.Fatalf("MultiGet must return unprefixed keys: %v", got)
			}

			if err := ns.SetWithTTL(ctx, "ttl", []byte("v"), time.Minute); err != nil {
				t.Fatalf("SetWithTTL: %v", err)
			}
			if err := ns.MultiSetWithTTL(ctx, map[string][]byte{"ttl2": []byte("v")}, time.Minute); err != nil {
				t.Fatalf("MultiSetWithTTL: %v", err)
			}
			for _, key := range []string{"ns:ttl", "ns:ttl2"} {
				if _, found, err := backend.Get(ctx, key); err != nil {
					t.Fatalf("backend.Get %s: %v", key, err)
				} else if !found {
					t.Fatalf("%s must be stored under the namespace prefix", key)
				}
			}

			val, found, err := ns.GetDel(ctx, "a")
			if err != nil {
				t.Fatalf("GetDel: %v", err)
			}
			if !found || string(val) != "1" {
				t.Fatalf("GetDel mismatch: found=%v val=%q", found, string(val))
			}
			if found, err := ns.Exists(ctx, "a"); err != nil {
				t.Fatalf("Exists after GetDel: %v", err)
			} else if found {
				t.Fatalf("GetDel must remove the key")
			}

			if err := ns.Del(ctx, "b"); err != nil {
				t.Fatalf("Del: %v", err)
			}
			if _, found, err := backend.Get(ctx, "ns:b"); err != nil {
				t.Fatalf("backend.Get after Del: %v", err)
			} else if found {
				t.Fatalf("Del must remove the prefixed key")
			}
		})
	}
}

// TestNSCacheDelAllOverNoCache pins that turning caching off keeps DelAll
// working. NSCache refuses DelAll without a KeyLister so it cannot wipe a
// sibling namespace's keys — but that risk cannot exist over a backend holding
// no keys at all, and nocache is documented as a drop-in switch, so a startup or
// admin path calling DelAll used to start failing the moment caching was
// disabled by configuration.
func TestNSCacheDelAllOverNoCache(t *testing.T) {
	ctx := context.Background()
	ns := nscache.NewNSCache[[]byte]("ns", nocache.NewByteNoCache())

	if err := ns.DelAll(ctx); err != nil {
		t.Fatalf("DelAll over nocache: %v", err)
	}
	keys, err := ns.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys over nocache: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("Keys over nocache = %v, want empty", keys)
	}
}
