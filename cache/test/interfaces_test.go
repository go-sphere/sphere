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
	"github.com/go-sphere/sphere/test/redistest"
)

var (
	_ cache.ByteCache           = (*memory.ByteCache)(nil)
	_ cache.ByteCache           = (*mcache.Map[string, []byte])(nil)
	_ cache.ByteCache           = (*badgerdb.Database)(nil)
	_ cache.ByteCache           = (*nocache.ByteNoCache)(nil)
	_ cache.ByteCache           = (*redis.ByteCache)(nil)
	_ cache.Cache[string]       = (*memory.Cache[string])(nil)
	_ cache.Cache[string]       = (*mcache.Map[string, string])(nil)
	_ cache.Cache[string]       = (*nocache.NoCache[string])(nil)
	_ cache.ExpirableByteCache  = (*memory.ByteCache)(nil)
	_ cache.ExpirableByteCache  = (*mcache.Map[string, []byte])(nil)
	_ cache.ExpirableByteCache  = (*badgerdb.Database)(nil)
	_ cache.ExpirableByteCache  = (*nocache.ByteNoCache)(nil)
	_ cache.ExpirableCache[int] = (*memory.Cache[int])(nil)
)

func TestRedisTypedCacheImplementsContract(t *testing.T) {
	t.Parallel()

	client := redistest.NewTestRedisClient(t)
	typed := redis.NewCache[string](client, codec.JsonCodec())

	if typed.GetByteCache() == nil {
		t.Fatalf("GetByteCache returned nil")
	}
	if typed.GetCodec() == nil {
		t.Fatalf("GetCodec returned nil")
	}

	var _ cache.Cache[string] = typed
}
