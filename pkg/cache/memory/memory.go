package memory

import (
	"context"
	"github.com/patrickmn/go-cache"
	"time"
)

type Cache struct {
	cache *cache.Cache
}

func NewMemoryCache(size int) *Cache {
	return &Cache{
		cache: cache.New(cache.NoExpiration, cache.NoExpiration),
	}
}

func (m *Cache) Set(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	m.cache.Set(key, val, expiration)
	return nil
}

func (m *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	v, exist := m.cache.Get(key)
	if !exist {
		return nil, nil
	}
	return v.([]byte), nil
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

func (m *Cache) Del(ctx context.Context, key string) error {
	m.cache.Delete(key)
	return nil
}

func (m *Cache) DelAll(ctx context.Context) error {
	m.cache.Flush()
	return nil
}

func (m *Cache) Close() error {
	return nil
}
