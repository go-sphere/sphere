package test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/cache/nocache"
	"github.com/go-sphere/sphere/cache/nscache"
	"github.com/go-sphere/sphere/cache/redis"
)

var (
	_ cache.ByteCache           = (*memory.ByteCache)(nil)
	_ cache.ByteCache           = (*mcache.Map[string, []byte])(nil)
	_ cache.ByteCache           = (*badgerdb.Database)(nil)
	_ cache.ByteCache           = (*nocache.ByteNoCache)(nil)
	_ cache.ByteCache           = (*redis.ByteCache)(nil)
	_ cache.Cache[string]       = (*cache.CodecCache[string])(nil)
	_ cache.Cache[string]       = (*memory.Cache[string])(nil)
	_ cache.Cache[string]       = (*mcache.Map[string, string])(nil)
	_ cache.Cache[string]       = (*nocache.NoCache[string])(nil)
	_ cache.ExpirableByteCache  = (*memory.ByteCache)(nil)
	_ cache.ExpirableByteCache  = (*mcache.Map[string, []byte])(nil)
	_ cache.ExpirableByteCache  = (*badgerdb.Database)(nil)
	_ cache.ExpirableByteCache  = (*nocache.ByteNoCache)(nil)
	_ cache.ExpirableCache[int] = (*memory.Cache[int])(nil)
	_ cache.KeyLister           = (*cache.CodecCache[string])(nil)
	_ cache.KeyLister           = (*nscache.NSCache[string])(nil)
	_ cache.KeyLister           = (*mcache.Map[string, []byte])(nil)
	_ cache.KeyLister           = (*badgerdb.Database)(nil)
	_ cache.KeyLister           = (*redis.ByteCache)(nil)
)

func TestCodecCacheImplementsContract(t *testing.T) {
	t.Parallel()

	typed := cache.NewCodecCache[string](mcache.NewByteCache(), codec.JsonCodec())

	if typed.GetByteCache() == nil {
		t.Fatalf("GetByteCache returned nil")
	}
	if typed.GetCodec() == nil {
		t.Fatalf("GetCodec returned nil")
	}

	var _ cache.Cache[string] = typed
}

// TestWrapperCloseLeavesBackendOpen pins the ownership rule at the wrapper
// layer: CodecCache and NSCache only ever receive an injected cache, so
// closing one must leave the backend usable for its owner and for any sibling
// wrapper. badgerdb is used because its Close is observable — mcache.Close is
// already a no-op and could not detect a regression here.
func TestWrapperCloseLeavesBackendOpen(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	backend, err := badgerdb.NewDatabase(badgerdb.Config{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("create badgerdb: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	typed := cache.NewJsonCache[string](backend)
	ns := nscache.NewNSCache[[]byte]("ns", backend)

	if err := backend.Set(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := typed.Close(); err != nil {
		t.Fatalf("CodecCache.Close: %v", err)
	}
	if err := ns.Close(); err != nil {
		t.Fatalf("NSCache.Close: %v", err)
	}

	val, found, err := backend.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get after wrapper Close: %v", err)
	}
	if !found || string(val) != "v" {
		t.Fatalf("wrapper Close closed the injected backend: found=%v value=%q", found, string(val))
	}
}

// TestCodecCacheTTLAndDelete exercises the adapter methods that the encode /
// decode round trip does not cover on its own: the TTL writes and the three
// delete paths.
func TestCodecCacheTTLAndDelete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	typed := cache.NewJsonCache[int](mcache.NewByteCache())

	if err := typed.SetWithTTL(ctx, "a", 1, time.Minute); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	if err := typed.MultiSetWithTTL(ctx, map[string]int{"b": 2, "c": 3}, time.Minute); err != nil {
		t.Fatalf("MultiSetWithTTL: %v", err)
	}

	got, found, err := typed.GetDel(ctx, "a")
	if err != nil {
		t.Fatalf("GetDel: %v", err)
	}
	if !found || got != 1 {
		t.Fatalf("GetDel mismatch: found=%v got=%d", found, got)
	}
	if found, err := typed.Exists(ctx, "a"); err != nil {
		t.Fatalf("Exists after GetDel: %v", err)
	} else if found {
		t.Fatalf("GetDel must remove the key")
	}

	if err := typed.Del(ctx, "b"); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if found, err := typed.Exists(ctx, "b"); err != nil {
		t.Fatalf("Exists after Del: %v", err)
	} else if found {
		t.Fatalf("Del must remove the key")
	}

	if err := typed.DelAll(ctx); err != nil {
		t.Fatalf("DelAll: %v", err)
	}
	if found, err := typed.Exists(ctx, "c"); err != nil {
		t.Fatalf("Exists after DelAll: %v", err)
	} else if found {
		t.Fatalf("DelAll must remove every key")
	}
}

func TestCodecCacheGetDelUnmarshalErrorConsumesKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	inner := mcache.NewByteCache()
	typed := cache.NewJsonCache[int](inner)
	if err := inner.Set(ctx, "k", []byte("not-json")); err != nil {
		t.Fatalf("Set raw: %v", err)
	}

	_, found, err := typed.GetDel(ctx, "k")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
	if !found {
		t.Fatal("found should be true because GetDel consumed an existing entry")
	}
	exists, existsErr := inner.Exists(ctx, "k")
	if existsErr != nil {
		t.Fatalf("Exists: %v", existsErr)
	}
	if exists {
		t.Fatal("undecodable key must be consumed by GetDel")
	}
}

type getBarrierByteCache struct {
	cache.ByteCache
	readCount atomic.Int32
	readTotal int32
	allRead   chan struct{}
}

func (c *getBarrierByteCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, found, err := c.ByteCache.Get(ctx, key)
	if c.readCount.Add(1) == c.readTotal {
		close(c.allRead)
	}
	<-c.allRead
	return value, found, err
}

func TestCodecCacheGetDelIsAtomic(t *testing.T) {
	t.Parallel()

	const consumers = 8
	inner := &getBarrierByteCache{
		ByteCache: mcache.NewByteCache(),
		readTotal: consumers,
		allRead:   make(chan struct{}),
	}
	typed := cache.NewJsonCache[int](inner)
	ctx := t.Context()
	if err := typed.Set(ctx, "once", 42); err != nil {
		t.Fatalf("Set: %v", err)
	}

	type result struct {
		found bool
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, consumers)
	for range consumers {
		go func() {
			<-start
			_, found, err := typed.GetDel(ctx, "once")
			results <- result{found: found, err: err}
		}()
	}
	close(start)

	hits := 0
	for range consumers {
		result := <-results
		if result.err != nil {
			t.Fatalf("GetDel: %v", result.err)
		}
		if result.found {
			hits++
		}
	}
	if hits != 1 {
		t.Fatalf("GetDel returned the same entry to %d callers, want 1", hits)
	}
}

func TestJsonCacheAdapter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	byteCache := mcache.NewByteCache()
	typed := cache.NewJsonCache[map[string]int](byteCache)

	if typed.GetByteCache() == nil {
		t.Fatalf("GetByteCache returned nil")
	}
	if typed.GetCodec() == nil {
		t.Fatalf("GetCodec returned nil")
	}

	if err := typed.Set(ctx, "k", map[string]int{"v": 1}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, found, err := typed.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found || got["v"] != 1 {
		t.Fatalf("Get mismatch: found=%v got=%v", found, got)
	}

	if err := typed.MultiSet(ctx, map[string]map[string]int{
		"k2": {"v": 2},
		"k3": {"v": 3},
	}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}
	m, err := typed.MultiGet(ctx, []string{"k2", "k3"})
	if err != nil {
		t.Fatalf("MultiGet: %v", err)
	}
	if m["k2"]["v"] != 2 || m["k3"]["v"] != 3 {
		t.Fatalf("MultiGet mismatch: %v", m)
	}
}
