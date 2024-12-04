package redis

import (
	"bytes"
	"context"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"
)

func setupTestByteCache(t *testing.T) (*ByteCache, func()) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for testing
	})

	// Check if Redis is available
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server is not available:", err)
	}

	cache := NewByteCache(client)

	cleanup := func() {
		_ = cache.DelAll(ctx)
		_ = cache.Close()
	}

	return cache, cleanup
}

func TestByteCache_Set_Get(t *testing.T) {
	cache, cleanup := setupTestByteCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test_key"
	value := []byte("test_value")

	// Test Set
	if err := cache.Set(ctx, key, value, time.Minute); err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Test Get
	result, err := cache.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if result == nil {
		t.Errorf("Get returned nil result")
	} else if !bytes.Equal(*result, value) {
		t.Errorf("Get returned wrong value: got %v want %v", *result, value)
	}

	// Test Get non-existent key
	result, err = cache.Get(ctx, "non_existent_key")
	if err == nil {
		t.Error("Expected error when getting non-existent key")
	}
	if result != nil {
		t.Error("Expected nil result when getting non-existent key")
	}
}

func TestByteCache_Del(t *testing.T) {
	cache, cleanup := setupTestByteCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test_key"
	value := []byte("test_value")

	// Set up test data
	if err := cache.Set(ctx, key, value, time.Minute); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test Del
	if err := cache.Del(ctx, key); err != nil {
		t.Errorf("Del failed: %v", err)
	}

	// Verify deletion
	result, err := cache.Get(ctx, key)
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}
	if result != nil {
		t.Error("Expected nil result when getting deleted key")
	}
}

func TestByteCache_MultiSet_MultiGet(t *testing.T) {
	cache, cleanup := setupTestByteCache(t)
	defer cleanup()

	ctx := context.Background()
	valMap := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	// Test MultiSet
	if err := cache.MultiSet(ctx, valMap, time.Minute); err != nil {
		t.Errorf("MultiSet failed: %v", err)
	}

	// Test MultiGet
	keys := []string{"key1", "key2", "key3", "non_existent_key"}
	results, err := cache.MultiGet(ctx, keys)
	if err != nil {
		t.Errorf("MultiGet failed: %v", err)
	}

	// Verify results
	for key, expected := range valMap {
		if got, ok := results[key]; !ok {
			t.Errorf("MultiGet missing key %s", key)
		} else if !bytes.Equal(got, expected) {
			t.Errorf("MultiGet wrong value for key %s: got %v want %v", key, got, expected)
		} else {
			t.Logf("MultiGet key %s: got %v", key, string(got))
		}
	}
	if _, ok := results["non_existent_key"]; ok {
		t.Error("MultiGet should not return non-existent key")
	}
}

func TestByteCache_MultiDel(t *testing.T) {
	cache, cleanup := setupTestByteCache(t)
	defer cleanup()

	ctx := context.Background()
	valMap := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	// Set up test data
	if err := cache.MultiSet(ctx, valMap, time.Minute); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test MultiDel
	keys := []string{"key1", "key2"}
	if err := cache.MultiDel(ctx, keys); err != nil {
		t.Errorf("MultiDel failed: %v", err)
	}

	// Verify deletion
	results, err := cache.MultiGet(ctx, []string{"key1", "key2", "key3"})
	if err != nil {
		t.Errorf("Verification MultiGet failed: %v", err)
	}

	if _, ok := results["key1"]; ok {
		t.Error("key1 should be deleted")
	}
	if _, ok := results["key2"]; ok {
		t.Error("key2 should be deleted")
	}
	if _, ok := results["key3"]; !ok {
		t.Error("key3 should still exist")
	}
}

func TestByteCache_DelAll(t *testing.T) {
	cache, cleanup := setupTestByteCache(t)
	defer cleanup()

	ctx := context.Background()
	valMap := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	// Set up test data
	if err := cache.MultiSet(ctx, valMap, time.Minute); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test DelAll
	if err := cache.DelAll(ctx); err != nil {
		t.Errorf("DelAll failed: %v", err)
	}

	// Verify all data is deleted
	for key := range valMap {
		result, err := cache.Get(ctx, key)
		if err == nil {
			t.Errorf("Expected error when getting key %s after DelAll", key)
		}
		if result != nil {
			t.Errorf("Expected nil result when getting key %s after DelAll", key)
		}
	}
}
