package memory

import (
	"context"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache[string]()

	err := cache.SetWithTTL(ctx, "key1", "value1", time.Minute)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	val, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if val == nil {
		t.Errorf("Expected value not to be nil")
	} else if *val != "value1" {
		t.Errorf("Expected value1, got %v", *val)
	}

	// Test non-existent key
	val, err = cache.Get(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Get nonexistent failed: %v", err)
	}
	if val != nil {
		t.Error("Expected nil for nonexistent key")
	}
}

func TestCache_Del(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache[string]()

	err := cache.SetWithTTL(ctx, "key2", "value2", time.Minute)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	err = cache.Del(ctx, "key2")
	if err != nil {
		t.Errorf("Del failed: %v", err)
	}

	val, err := cache.Get(ctx, "key2")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if val != nil {
		t.Error("Expected nil after deletion")
	}
}

func TestCache_MultiSetAndGet(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache[string]()

	valMap := map[string]string{
		"key3": "value3",
		"key4": "value4",
	}
	err := cache.MultiSetWithTTL(ctx, valMap, time.Minute)
	if err != nil {
		t.Errorf("MultiSet failed: %v", err)
	}

	results, err := cache.MultiGet(ctx, []string{"key3", "key4", "nonexistent"})
	if err != nil {
		t.Errorf("MultiGet failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if results["key3"] != "value3" {
		t.Errorf("Expected value3, got %v", results["key3"])
	}
	if results["key4"] != "value4" {
		t.Errorf("Expected value4, got %v", results["key4"])
	}
}

func TestCache_MultiDel(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache[string]()

	valMap := map[string]string{
		"key5": "value5",
		"key6": "value6",
	}
	err := cache.MultiSetWithTTL(ctx, valMap, time.Minute)
	if err != nil {
		t.Errorf("MultiSet failed: %v", err)
	}

	err = cache.MultiDel(ctx, []string{"key5", "key6"})
	if err != nil {
		t.Errorf("MultiDel failed: %v", err)
	}

	results, err := cache.MultiGet(ctx, []string{"key5", "key6"})
	if err != nil {
		t.Errorf("MultiGet failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results after deletion, got %d", len(results))
	}
}

func TestCache_DelAll(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache[string]()

	err := cache.SetWithTTL(ctx, "key7", "value7", time.Minute)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	err = cache.DelAll(ctx)
	if err != nil {
		t.Errorf("DelAll failed: %v", err)
	}

	val, err := cache.Get(ctx, "key7")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if val != nil {
		t.Error("Expected nil after DelAll")
	}
}

func TestCache_Expiration(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache[string]()

	err := cache.SetWithTTL(ctx, "expiring", "value", 50*time.Millisecond)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Value should exist immediately
	val, err := cache.Get(ctx, "expiring")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if val == nil {
		t.Errorf("Expected value not to be nil")
	} else if *val != "value" {
		t.Errorf("Expected value, got %v", *val)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Value should be gone
	val, err = cache.Get(ctx, "expiring")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	} else if val != nil {
		t.Error("Expected nil for expired key")
	}
}

func TestCache_TypeSafety(t *testing.T) {
	ctx := context.Background()
	// Create a cache of different type
	intCache := NewMemoryCache[int]()
	err := intCache.SetWithTTL(ctx, "int", 123, time.Minute)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	val, err := intCache.Get(ctx, "int")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if val == nil {
		t.Errorf("Expected value not to be nil")
	} else if *val != 123 {
		t.Errorf("Expected 123, got %v", *val)
	}
}

func TestCache_ByteCache(t *testing.T) {
	ctx := context.Background()
	byteCache := NewByteCache()
	data := []byte("test data")

	err := byteCache.SetWithTTL(ctx, "bytes", data, time.Minute)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	val, err := byteCache.Get(ctx, "bytes")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if val == nil {
		t.Errorf("Expected value not to be nil")
	} else if string(*val) != string(data) {
		t.Errorf("Expected %s, got %s", string(data), string(*val))
	}
}

func TestCache_Close(t *testing.T) {
	cache := NewMemoryCache[string]()
	err := cache.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
