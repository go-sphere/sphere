package test

import (
	"context"
	"testing"
	"time"

	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/cache/badgerdb"
	"github.com/TBXark/sphere/cache/mcache"
	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/cache/redis"
	redisConn "github.com/TBXark/sphere/infra/redis"
)

func testCache(ctx context.Context, t *testing.T, byteCache cache.ByteCache) {
	// Set
	err := byteCache.Set(ctx, "testKey", []byte("testValue"))
	if err != nil {
		t.Errorf("Set failed: %v", err)
		return
	}
	t.Log("Set ok")

	// Get existing key
	bytes, found, err := byteCache.Get(ctx, "testKey")
	if err != nil {
		t.Errorf("Get after Set failed: %v", err)
		return
	}
	if !found {
		t.Errorf("Expected to find key 'testKey', but did not find it")
		return
	}
	if string(bytes) != "testValue" {
		t.Errorf("Expected value 'testValue', got: %s", bytes)
		return
	}
	t.Log("Get ok")

	// SetWithTTL
	err = byteCache.SetWithTTL(ctx, "testKeyTTL", []byte("testValueTTL"), time.Millisecond)
	if err != nil {
		t.Errorf("SetWithTTL failed: %v", err)
		return
	}
	time.Sleep(time.Millisecond * 10) // Wait for TTL to expire

	// Get non-existing key after TTL expiration
	bytes, found, err = byteCache.Get(ctx, "testKeyTTL")
	if err != nil {
		t.Errorf("Get after SetWithTTL failed: %v", err)
		return
	}
	if found {
		t.Errorf("Expected not to find key 'testKeyTTL' after TTL expiration, but found: %s", bytes)
		return
	}
	if len(bytes) != 0 {
		t.Errorf("Expected empty bytes, got: %s", bytes)
		return
	}
	t.Log("SetWithTTL and Get after TTL ok")

	// MultiSet
	err = byteCache.MultiSet(ctx, map[string][]byte{
		"testKey1": []byte("testValue1"),
		"testKey2": []byte("testValue2"),
		"testKey3": []byte("testValue3"),
	})
	if err != nil {
		t.Errorf("MultiSet failed: %v", err)
		return
	}
	t.Log("MultiSet ok")

	// MultiGet
	multiBytes, err := byteCache.MultiGet(ctx, []string{"testKey1", "testKey2"})
	if err != nil {
		t.Errorf("MultiGet failed: %v", err)
		return
	}
	if string(multiBytes["testKey1"]) != "testValue1" || string(multiBytes["testKey2"]) != "testValue2" {
		t.Errorf("Expected MultiGet to return 'testValue1' and 'testValue2', got: %s and %s", multiBytes["testKey1"], multiBytes["testKey2"])
		return
	}
	t.Log("MultiGet ok")

	// Del
	err = byteCache.Del(ctx, "testKey1")
	if err != nil {
		t.Errorf("Del failed: %v", err)
		return
	}
	found, err = byteCache.Exists(ctx, "testKey1")
	if err != nil {
		t.Errorf("Get after Del failed: %v", err)
		return
	}
	if found {
		t.Errorf("Expected not to find key 'testKey1' after Del, but found it")
		return
	}
	t.Log("Del ok")

	// DelMulti
	err = byteCache.MultiDel(ctx, []string{"testKey2", "testKey3"})
	if err != nil {
		t.Errorf("MultiDel failed: %v", err)
		return
	}
	found, err = byteCache.Exists(ctx, "testKey2")
	if err != nil {
		t.Errorf("Exists after MultiDel failed: %v", err)
		return
	}
	if found {
		t.Errorf("Expected not to find key 'testKey2' after MultiDel, but found it")
		return
	}
	t.Log("MultiDel ok")

	// DelAll
	err = byteCache.DelAll(ctx)
	if err != nil {
		t.Errorf("DelAll failed: %v", err)
		return
	}
	found, err = byteCache.Exists(ctx, "testKey3")
	if err != nil {
		t.Errorf("Exists after DelAll failed: %v", err)
		return
	}
	if found {
		t.Errorf("Expected not to find key 'testKey3' after DelAll, but found it")
		return
	}
	t.Log("DelAll ok")

	// MultiSetWithTTL
	err = byteCache.MultiSetWithTTL(ctx, map[string][]byte{
		"testKey4": []byte("testValue4"),
		"testKey5": []byte("testValue5"),
	}, time.Millisecond)
	if err != nil {
		t.Errorf("MultiSetWithTTL failed: %v", err)
		return
	}
	time.Sleep(time.Millisecond * 10) // Wait for TTL to expire
	found, err = byteCache.Exists(ctx, "testKey4")
	if err != nil {
		t.Errorf("Exists after MultiSetWithTTL failed: %v", err)
		return
	}
	if found {
		t.Errorf("Expected not to find key 'testKey4' after MultiSetWithTTL expiration, but found it")
		return
	}
	t.Log("MultiSetWithTTL ok")
}

func TestRedisCache(t *testing.T) {
	client, err := redisConn.NewClient(&redisConn.Config{
		URL: "redis://localhost:6379/0",
	})
	if err != nil {
		t.Skipf("Redis server not available, skipping test: %v", err)
	}
	db := redis.NewByteCache(client)
	testCache(context.Background(), t, db)
}

func TestMemoryCache(t *testing.T) {
	testCache(context.Background(), t, memory.NewByteCache())
}

func TestMCache(t *testing.T) {
	testCache(context.Background(), t, mcache.NewMapCache[[]byte]())
}

func TestBadgerDBCache(t *testing.T) {
	db, err := badgerdb.NewDatabase(&badgerdb.Config{Path: "./temp"})
	if err != nil {
		t.Skip("Skipping BadgerDB test, could not create temp file:", err)
	}
	testCache(context.Background(), t, db)
}
