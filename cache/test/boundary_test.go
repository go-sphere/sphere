package test

import (
	"context"
	"testing"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/core/safe"
)

// testBoundaryConditions tests edge cases like empty keys, nil values, and large values
func testBoundaryConditions(ctx context.Context, t *testing.T, cache cache.ByteCache, name string) {
	defer safe.IfErrorPresent(cache.Close)

	// Test empty string key (some implementations may not support this)
	err := cache.Set(ctx, "", []byte("emptyKeyValue"))
	if err != nil {
		// Some implementations like BadgerDB don't support empty keys
		t.Logf("%s: Empty key not supported (acceptable): %v", name, err)
	} else {
		val, found, err := cache.Get(ctx, "")
		if err != nil {
			t.Errorf("%s: Get with empty key failed: %v", name, err)
			return
		}
		if !found {
			t.Errorf("%s: Expected to find empty key", name)
			return
		}
		if string(val) != "emptyKeyValue" {
			t.Errorf("%s: Expected 'emptyKeyValue', got: %s", name, val)
			return
		}
	}

	// Test nil value (empty byte slice)
	err = cache.Set(ctx, "nilKey", nil)
	if err != nil {
		t.Errorf("%s: Set with nil value failed: %v", name, err)
		return
	}

	val, found, err := cache.Get(ctx, "nilKey")
	if err != nil {
		t.Errorf("%s: Get with nil value failed: %v", name, err)
		return
	}
	if !found {
		t.Errorf("%s: Expected to find nil value key", name)
		return
	}
	if len(val) != 0 {
		t.Errorf("%s: Expected nil or empty slice, got: %v", name, val)
		return
	}

	// Test large value
	largeValue := make([]byte, 1024*1024) // 1MB
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	err = cache.Set(ctx, "largeKey", largeValue)
	if err != nil {
		t.Errorf("%s: Set with large value failed: %v", name, err)
		return
	}

	val, found, err = cache.Get(ctx, "largeKey")
	if err != nil {
		t.Errorf("%s: Get with large value failed: %v", name, err)
		return
	}
	if !found {
		t.Errorf("%s: Expected to find large value key", name)
		return
	}
	if len(val) != len(largeValue) {
		t.Errorf("%s: Large value length mismatch: expected %d, got %d", name, len(largeValue), len(val))
		return
	}

	t.Logf("%s: Boundary conditions test ok", name)
}

func TestMemoryCacheBoundaryConditions(t *testing.T) {
	byteCache := memory.NewByteCache()
	testBoundaryConditions(context.Background(), t, byteCache, "Memory cache")
}

func TestMCacheBoundaryConditions(t *testing.T) {
	byteCache := mcache.NewMapCache[[]byte]()
	testBoundaryConditions(context.Background(), t, byteCache, "MCache")
}

func TestBadgerDBCacheBoundaryConditions(t *testing.T) {
	db, err := badgerdb.NewDatabase(&badgerdb.Config{Path: "./temp_boundary"})
	if err != nil {
		t.Skip("Skipping BadgerDB boundary test:", err)
	}
	testBoundaryConditions(context.Background(), t, db, "BadgerDB")
}
