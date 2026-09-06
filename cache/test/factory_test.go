// Package test is the cross-driver contract suite for cache.Cache.
//
// Every ByteCache driver (memory, mcache, redis, badgerdb, nocache) is
// registered here and run against the same cases: CRUD, TTL (including
// zero/negative), batches, GetDel, Close, and KeyLister where supported.
// Add a new driver by returning it from statefulByteCacheFactories rather
// than writing a parallel suite.
package test

import (
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

type byteCacheFactory struct {
	name string
	new  func(tb testing.TB) cache.ByteCache
}

func statefulByteCacheFactories() []byteCacheFactory {
	return []byteCacheFactory{
		{
			name: "memory",
			new: func(tb testing.TB) cache.ByteCache {
				tb.Helper()
				c := memory.NewByteCache()
				tb.Cleanup(func() { _ = c.Close() })
				return c
			},
		},
		{
			name: "mcache",
			new: func(tb testing.TB) cache.ByteCache {
				tb.Helper()
				c := mcache.NewByteCache()
				tb.Cleanup(func() { _ = c.Close() })
				return c
			},
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
		},
		{
			name: "redis",
			new: func(tb testing.TB) cache.ByteCache {
				t, ok := tb.(*testing.T)
				if !ok {
					tb.Fatalf("redis test factory requires *testing.T")
				}
				t.Helper()
				client := redistest.NewTestRedisClient(t)
				c := redis.NewByteCache(client)
				tb.Cleanup(func() { _ = c.Close() })
				return c
			},
		},
		{
			// nscache wraps an injected backend; its Close is a no-op, so the
			// wrapped mcache (the only in-memory driver implementing KeyLister,
			// which DelAll requires) is owned and closed by the factory.
			// Registering it here pins that its namespace wrapping
			// (DelAll/Keys/MultiDel) honours the same contract as the plain
			// drivers.
			name: "nscache(mcache)",
			new: func(tb testing.TB) cache.ByteCache {
				tb.Helper()
				backend := mcache.NewByteCache()
				tb.Cleanup(func() { _ = backend.Close() })
				return nscache.NewNSCache[[]byte]("contract-ns", backend)
			},
		},
	}
}

func noCacheFactory() byteCacheFactory {
	return byteCacheFactory{
		name: "nocache",
		new: func(tb testing.TB) cache.ByteCache {
			tb.Helper()
			c := nocache.NewByteNoCache()
			tb.Cleanup(func() { _ = c.Close() })
			return c
		},
	}
}
