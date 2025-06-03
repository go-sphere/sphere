package memory

import (
	"context"
	"errors"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

type Cache[T any] struct {
	calculateCost    bool
	allowAsyncWrites bool
	cache            *ristretto.Cache[string, T]
}

func NewMemoryCache[T any]() *Cache[T] {
	cache, _ := ristretto.NewCache[string, T](&ristretto.Config[string, T]{
		NumCounters: 1e7,
		MaxCost:     1 << 10,
		BufferItems: 64,
	})
	return &Cache[T]{
		cache:         cache,
		calculateCost: false,
	}
}

func NewMemoryCacheWithCost[T any](cost func(T) int64) *Cache[T] {
	cache, _ := ristretto.NewCache[string, T](&ristretto.Config[string, T]{
		NumCounters: 1e7,
		MaxCost:     1 << 10,
		BufferItems: 64,
		Cost:        cost,
	})
	return &Cache[T]{
		cache:         cache,
		calculateCost: true,
	}
}

// UpdateMaxCost  In memory.Cache, By default, `calculateCost` is False, so `cost` will be 1.
// It doesn't care about the size of the item.
// Calculating cost is too complex and not necessary for most use cases.
// If you want to limit the number of items in the cache, you use this method to set the maximum number of items.
// If you want to limit the size of the items in the cache, you can use NewMemoryCacheWithCost
func (m *Cache[T]) UpdateMaxCost(maxItem int64) {
	if maxItem > 0 {
		m.cache.UpdateMaxCost(maxItem)
	}
}

// SetAllowAsyncWrites In memory.Cache asynchronous writes are disabled by default.
// If asynchronous writes are enabled, the cache will not block the Set method
// But it will not guarantee that the value is written to the cache immediately.
func (m *Cache[T]) SetAllowAsyncWrites(allow bool) {
	m.allowAsyncWrites = allow
}

// Set sets a value in the cache with an optional expiration time.
// If expiration is less than zero, the value will not expire.
func (m *Cache[T]) Set(ctx context.Context, key string, val T, expiration time.Duration) error {
	var success bool
	var cost int64 = 1
	if m.calculateCost {
		cost = 0
	}
	if expiration < 0 {
		success = m.cache.Set(key, val, cost)
	} else {
		success = m.cache.SetWithTTL(key, val, cost, expiration)
	}
	if !success {
		return errors.New("cache set failed")
	}
	if !m.allowAsyncWrites {
		m.cache.Wait()
	}
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
	return nil
}

func (m *Cache[T]) DelAll(ctx context.Context) error {
	m.cache.Clear()
	return nil
}

func (m *Cache[T]) Close() error {
	m.cache.Close()
	return nil
}

type ByteCache = Cache[[]byte]

func NewByteCache() *ByteCache {
	return NewMemoryCacheWithCost[[]byte](func(bytes []byte) int64 {
		return int64(len(bytes))
	})
}
