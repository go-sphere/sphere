package test

import (
	"testing"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/cache/nocache"
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
