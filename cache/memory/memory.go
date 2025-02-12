package memory

import (
	"context"
	"fmt"
	"github.com/patrickmn/go-cache"
	"time"
)

var (
	ErrorType = fmt.Errorf("type error")
)

type Cache[T any] struct {
	cache *cache.Cache
}

func NewMemoryCache[T any]() *Cache[T] {
	return &Cache[T]{
		cache: cache.New(cache.NoExpiration, cache.NoExpiration),
	}
}

func (m *Cache[T]) Set(ctx context.Context, key string, val T, expiration time.Duration) error {
	m.cache.Set(key, val, expiration)
	return nil
}

func (m *Cache[T]) Get(ctx context.Context, key string) (*T, error) {
	v, exist := m.cache.Get(key)
	if !exist {
		return nil, nil
	}
	val, ok := v.(T)
	if !ok {
		return nil, ErrorType
	}
	return &val, nil
}

func (m *Cache[T]) Del(ctx context.Context, key string) error {
	m.cache.Delete(key)
	return nil
}

func (m *Cache[T]) MultiSet(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	for k, v := range valMap {
		err := m.Set(ctx, k, v, expiration)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Cache[T]) MultiGet(ctx context.Context, keys []string) (map[string]T, error) {
	result := make(map[string]T)
	for _, key := range keys {
		val, err := m.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if val != nil {
			result[key] = *val
		}
	}
	return result, nil
}

func (m *Cache[T]) MultiDel(ctx context.Context, keys []string) error {
	for _, key := range keys {
		err := m.Del(ctx, key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Cache[T]) DelAll(ctx context.Context) error {
	m.cache.Flush()
	return nil
}

func (m *Cache[T]) Close() error {
	return nil
}

func NewByteCache() *Cache[[]byte] {
	return NewMemoryCache[[]byte]()
}
