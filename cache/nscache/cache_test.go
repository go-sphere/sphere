package nscache_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
			supportsListing: false,
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
		if factory.supportsListing {
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
