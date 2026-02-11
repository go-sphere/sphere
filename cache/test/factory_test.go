package test

import (
	"testing"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/cache/nocache"
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
