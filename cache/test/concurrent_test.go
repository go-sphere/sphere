package test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/core/safe"
)

// testConcurrentAccess tests concurrent read/write operations
func testConcurrentAccess(ctx context.Context, t *testing.T, cache cache.ByteCache, name string) {
	defer safe.IfErrorPresent(cache.Close)

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				if err := cache.Set(ctx, key, []byte(value)); err != nil {
					errors <- fmt.Errorf("goroutine %d: Set failed: %v", id, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify written data
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numOperations; j++ {
			key := fmt.Sprintf("key_%d_%d", i, j)
			expectedValue := fmt.Sprintf("value_%d_%d", i, j)

			val, found, err := cache.Get(ctx, key)
			if err != nil {
				t.Errorf("%s: Get failed for key %s: %v", name, key, err)
				continue
			}
			if !found {
				t.Errorf("%s: Key %s not found", name, key)
				continue
			}
			if string(val) != expectedValue {
				t.Errorf("%s: Value mismatch for key %s: expected %s, got %s", name, key, expectedValue, val)
			}
		}
	}

	// Concurrent reads on same keys
	wg = sync.WaitGroup{}
	errors = make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id%numGoroutines, j%numOperations)
				_, found, err := cache.Get(ctx, key)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: Get failed: %v", id, err)
					return
				}
				if !found {
					errors <- fmt.Errorf("goroutine %d: Key %s not found during concurrent read", id, key)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	t.Logf("%s: Concurrent access test completed successfully", name)
}

func TestMemoryCacheConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent access test in short mode")
	}

	byteCache := memory.NewByteCache()
	testConcurrentAccess(context.Background(), t, byteCache, "Memory cache")
}

func TestMCacheConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent access test in short mode")
	}

	byteCache := mcache.NewMapCache[[]byte]()
	testConcurrentAccess(context.Background(), t, byteCache, "MCache")
}

func TestBadgerDBCacheConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent access test in short mode")
	}

	// Limit concurrent access for BadgerDB to avoid file descriptor issues
	oldMaxProcs := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(oldMaxProcs)

	db, err := badgerdb.NewDatabase(&badgerdb.Config{Path: "./temp_concurrent"})
	if err != nil {
		t.Skip("Skipping BadgerDB concurrent test:", err)
	}
	testConcurrentAccess(context.Background(), t, db, "BadgerDB")
}
