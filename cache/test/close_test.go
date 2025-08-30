package test

import (
	"testing"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/cache/nocache"
)

// testCloseMethod tests the Close method of a cache implementation
func testCloseMethod(t *testing.T, cache cache.ByteCache, name string) {
	err := cache.Close()
	if err != nil {
		t.Errorf("%s Close failed: %v", name, err)
	}
}

func TestMemoryCacheClose(t *testing.T) {
	byteCache := memory.NewByteCache()
	testCloseMethod(t, byteCache, "Memory cache")
}

func TestMCacheClose(t *testing.T) {
	byteCache := mcache.NewMapCache[[]byte]()
	testCloseMethod(t, byteCache, "MCache")
}

func TestNoCacheClose(t *testing.T) {
	byteCache := nocache.NewByteNoCache()
	testCloseMethod(t, byteCache, "NoCache")
}

func TestBadgerDBCacheClose(t *testing.T) {
	db, err := badgerdb.NewDatabase(&badgerdb.Config{Path: "./temp_close"})
	if err != nil {
		t.Skip("Skipping BadgerDB close test:", err)
	}
	testCloseMethod(t, db, "BadgerDB")
}
