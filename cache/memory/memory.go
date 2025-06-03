package memory

import (
	"context"
	"errors"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

type Cache[T any] struct {
	cache *ristretto.Cache[string, T]
}

func NewMemoryCache[T any]() *Cache[T] {
	cache, _ := ristretto.NewCache[string, T](&ristretto.Config[string, T]{
		NumCounters: 1e7,
		MaxCost:     1 << 30,
		BufferItems: 64,
	})
	return &Cache[T]{
		cache: cache,
	}
}

func (m *Cache[T]) UpdateMaxCost(maxCost int64) {
	if maxCost > 0 {
		m.cache.UpdateMaxCost(maxCost)
	}
}

func (m *Cache[T]) Set(ctx context.Context, key string, val T, expiration time.Duration) error {
	success := m.cache.SetWithTTL(key, val, 1, expiration)
	if !success {
		return errors.New("cache set failed")
	}
	m.cache.Wait()
	return nil
}

func (m *Cache[T]) Get(ctx context.Context, key string) (*T, error) {
	val, found := m.cache.Get(key)
	if !found {
		return nil, nil
	}
	return &val, nil
}

func (m *Cache[T]) Del(ctx context.Context, key string) error {
	m.cache.Del(key)
	return nil
}

func (m *Cache[T]) MultiSet(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	var errs []error
	for k, v := range valMap {
		success := m.cache.SetWithTTL(k, v, 1, expiration)
		if !success {
			errs = append(errs, errors.New("cache set failed for key: "+k))
		}
	}
	m.cache.Wait()
	if len(errs) > 0 {
		return errors.Join(errs...)
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
		m.cache.Del(key)
	}
	m.cache.Wait()
	return nil
}

func (m *Cache[T]) DelAll(ctx context.Context) error {
	m.cache.Clear()
	m.cache.Wait()
	return nil
}

func (m *Cache[T]) Close() error {
	m.cache.Close()
	return nil
}

func NewByteCache() *Cache[[]byte] {
	return NewMemoryCache[[]byte]()
}
