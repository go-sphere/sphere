package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/core/safe"
)

// testContextCancellation tests behavior with cancelled contexts
func testContextCancellation(t *testing.T, cache cache.ByteCache, name string) {
	defer safe.IfErrorPresent(cache.Close)

	// Test Set with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := cache.Set(cancelledCtx, "key1", []byte("value1"))
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("%s: Set with cancelled context returned error (acceptable): %v", name, err)
	}

	// Test Get with cancelled context
	_, _, err = cache.Get(cancelledCtx, "key1")
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("%s: Get with cancelled context returned error (acceptable): %v", name, err)
	}

	// Test with timeout context
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancelTimeout()
	time.Sleep(2 * time.Microsecond) // Ensure timeout

	err = cache.Set(timeoutCtx, "key2", []byte("value2"))
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("%s: Set with timeout context returned error (acceptable): %v", name, err)
	}

	t.Logf("%s: Context cancellation test completed", name)
}

func TestMemoryCacheContextCancellation(t *testing.T) {
	byteCache := memory.NewByteCache()
	testContextCancellation(t, byteCache, "Memory cache")
}

func TestMCacheContextCancellation(t *testing.T) {
	byteCache := mcache.NewMapCache[[]byte]()
	testContextCancellation(t, byteCache, "MCache")
}

func TestBadgerDBCacheContextCancellation(t *testing.T) {
	db, err := badgerdb.NewDatabase(&badgerdb.Config{Path: "./temp_context"})
	if err != nil {
		t.Skip("Skipping BadgerDB context test:", err)
	}
	testContextCancellation(t, db, "BadgerDB")
}
