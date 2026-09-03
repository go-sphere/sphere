package test

import (
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
