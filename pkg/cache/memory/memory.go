package memory

import (
	"context"
	"github.com/coocood/freecache"
	"time"
)

type Cache struct {
	cache *freecache.Cache
}

func NewMemoryCache(size int) *Cache {
	return &Cache{
		cache: freecache.NewCache(size),
	}
}

func (m *Cache) Set(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	return m.cache.Set([]byte(key), val, int(expiration.Seconds()))
}

func (m *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	return m.cache.Get([]byte(key))
}

func (m *Cache) MultiSet(ctx context.Context, valMap map[string][]byte, expiration time.Duration) error {
	for k, v := range valMap {
		err := m.Set(ctx, k, v, expiration)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Cache) MultiGet(ctx context.Context, keys []string) (map[string][]byte, error) {
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

func (m *Cache) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		_ = m.cache.Del([]byte(key))
	}
	return nil
}

func (m *Cache) DelAll(ctx context.Context) error {
	m.cache.Clear()
	return nil
}
