package cache

import (
	"context"
	"github.com/coocood/freecache"
	"time"
)

type memoryCache struct {
	cache *freecache.Cache
}

func NewMemoryCache() ByteCache {
	return &memoryCache{
		cache: freecache.NewCache(1024 * 1024 * 1024),
	}
}

func (m *memoryCache) Set(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	return m.cache.Set([]byte(key), val, int(expiration.Seconds()))
}

func (m *memoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	return m.cache.Get([]byte(key))
}

func (m *memoryCache) MultiSet(ctx context.Context, valMap map[string][]byte, expiration time.Duration) error {
	for k, v := range valMap {
		err := m.Set(ctx, k, v, expiration)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *memoryCache) MultiGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, key := range keys {
		val, err := m.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		result[key] = val
	}
	return result, nil
}

func (m *memoryCache) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		_ = m.cache.Del([]byte(key))
	}
	return nil
}

func (m *memoryCache) DelAll(ctx context.Context) error {
	m.cache.Clear()
	return nil
}
