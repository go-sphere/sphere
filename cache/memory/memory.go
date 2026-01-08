package memory

import (
	"context"
	"errors"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

const (
	defaultMaxCost     = 1 << 30 // 1GB
	defaultNumCounters = 1e7
	defaultBufferItems = 64
)

// Cache is an in-memory cache implementation backed by ristretto that provides high-performance caching
// with configurable cost calculation and asynchronous write options.
type Cache[T any] struct {
	calculateCost    bool
	allowAsyncWrites bool
	cache            *ristretto.Cache[string, T]
}

// NewMemoryCache creates a new in-memory cache with default settings.
// The cache uses a fixed cost of 1 per item and does not calculate actual memory usage.
func NewMemoryCache[T any]() *Cache[T] {
	cache, _ := ristretto.NewCache[string, T](&ristretto.Config[string, T]{
		NumCounters: defaultNumCounters,
		MaxCost:     defaultMaxCost,
		BufferItems: defaultBufferItems,
	})
	return &Cache[T]{
		cache:         cache,
		calculateCost: false,
	}
}

// NewMemoryCacheWithCost creates a new in-memory cache with a custom cost function.
// The cost function determines the memory cost of each cached item, enabling memory-based eviction policies.
func NewMemoryCacheWithCost[T any](cost func(T) int64) *Cache[T] {
	cache, _ := ristretto.NewCache[string, T](&ristretto.Config[string, T]{
		NumCounters: defaultNumCounters,
		MaxCost:     defaultMaxCost,
		BufferItems: defaultBufferItems,
		Cost:        cost,
	})
	return &Cache[T]{
		cache:         cache,
		calculateCost: true,
	}
}

// NewMemoryCacheWithRistretto creates a new cache wrapper around an existing ristretto cache instance.
// This allows for advanced configuration and sharing of cache instances across multiple Cache wrappers.
func NewMemoryCacheWithRistretto[T any](cache *ristretto.Cache[string, T], calculateCost, allowAsyncWrites bool) *Cache[T] {
	return &Cache[T]{
		calculateCost:    calculateCost,
		allowAsyncWrites: allowAsyncWrites,
		cache:            cache,
	}
}

// UpdateMaxCost updates the maximum cost allowed for the cache.
// In memory.Cache, by default, `calculateCost` is False, so `cost` will be 1.
// It doesn't care about the size of the item.
// Calculating cost is too complex and not necessary for most use cases.
// If you want to limit the number of items in the cache, you use this method to set the maximum number of items.
// If you want to limit the size of the items in the cache, you can use NewMemoryCacheWithCost
func (m *Cache[T]) UpdateMaxCost(maxItem int64) {
	if maxItem > 0 {
		m.cache.UpdateMaxCost(maxItem)
	}
}

// SetAllowAsyncWrites configures whether the cache should use asynchronous writes.
// In memory.Cache asynchronous writes are disabled by default.
// If asynchronous writes are enabled, the cache will not block the Set method
// but it will not guarantee that the value is written to the cache immediately.
func (m *Cache[T]) SetAllowAsyncWrites(allow bool) {
	m.allowAsyncWrites = allow
}

func (m *Cache[T]) Set(ctx context.Context, key string, val T) error {
	var cost int64 = 1
	if m.calculateCost {
		cost = 0
	}
	if !m.cache.Set(key, val, cost) {
		return errors.New("cache set failed")
	}
	if !m.allowAsyncWrites {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	var cost int64 = 1
	if m.calculateCost {
		cost = 0
	}
	if !m.cache.SetWithTTL(key, val, cost, expiration) {
		return errors.New("cache set failed")
	}
	if !m.allowAsyncWrites {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) MultiSet(ctx context.Context, valMap map[string]T) error {
	var errs []error
	for k, v := range valMap {
		var cost int64 = 1
		if m.calculateCost {
			cost = 0
		}
		success := m.cache.Set(k, v, cost)
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

func (m *Cache[T]) MultiSetWithTTL(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	var errs []error
	for k, v := range valMap {
		var cost int64 = 1
		if m.calculateCost {
			cost = 0
		}
		success := m.cache.SetWithTTL(k, v, cost, expiration)
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

func (m *Cache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	val, found := m.cache.Get(key)
	return val, found, nil
}

func (m *Cache[T]) MultiGet(ctx context.Context, keys []string) (map[string]T, error) {
	result := make(map[string]T)
	for _, key := range keys {
		val, found := m.cache.Get(key)
		if found {
			result[key] = val
		}
	}
	return result, nil
}

func (m *Cache[T]) Del(ctx context.Context, key string) error {
	m.cache.Del(key)
	return nil
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

func (m *Cache[T]) Exists(ctx context.Context, key string) (bool, error) {
	_, found := m.cache.Get(key)
	return found, nil
}

func (m *Cache[T]) Close() error {
	m.cache.Close()
	return nil
}

// Wait blocks until all pending writes are processed.
func (m *Cache[T]) Sync() error {
	m.cache.Wait()
	return nil
}

type ByteCache = Cache[[]byte]

func NewByteCache() *ByteCache {
	return NewMemoryCacheWithCost[[]byte](func(bytes []byte) int64 {
		return int64(len(bytes))
	})
}
