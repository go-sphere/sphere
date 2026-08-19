package memory

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/go-sphere/sphere/cache"
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
	allowAsyncWrites atomic.Bool
	cache            *ristretto.Cache[string, T]
	// getDel serializes GetDel so an entry is handed to at most one caller.
	getDel sync.Mutex
	// owned reports whether this Cache created the underlying ristretto cache
	// and is therefore responsible for closing it. Injected instances are not
	// owned.
	owned bool
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
		owned:         true,
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
		owned:         true,
	}
}

// NewMemoryCacheWithRistretto creates a new cache wrapper around an existing ristretto cache instance.
// This allows for advanced configuration and sharing of cache instances across multiple Cache wrappers.
// The ristretto cache is injected, not owned: Close does not close it, so the
// caller keeps ownership and must close the *ristretto.Cache itself.
func NewMemoryCacheWithRistretto[T any](cache *ristretto.Cache[string, T], calculateCost, allowAsyncWrites bool) *Cache[T] {
	c := &Cache[T]{
		calculateCost: calculateCost,
		cache:         cache,
		owned:         false,
	}
	c.allowAsyncWrites.Store(allowAsyncWrites)
	return c
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
// It is safe to call concurrently with Set/MultiSet.
func (m *Cache[T]) SetAllowAsyncWrites(allow bool) {
	m.allowAsyncWrites.Store(allow)
}

// A false return from ristretto's Set/SetWithTTL does not signal a hard
// failure: under load the entry may be dropped when the internal setBuf is
// full. Cache semantics allow such losses, so a dropped write is reported as
// success (return nil) rather than a non-deterministic error that would push
// callers into retry storms. A ristretto ttl of 0 means "never expire", which
// matches the cache TTL contract; negative TTLs are rejected up front.

func (m *Cache[T]) Set(ctx context.Context, key string, val T) error {
	var cost int64 = 1
	if m.calculateCost {
		cost = 0
	}
	m.cache.Set(key, val, cost)
	if !m.allowAsyncWrites.Load() {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	var cost int64 = 1
	if m.calculateCost {
		cost = 0
	}
	m.cache.SetWithTTL(key, val, cost, expiration)
	if !m.allowAsyncWrites.Load() {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) MultiSet(ctx context.Context, valMap map[string]T) error {
	for k, v := range valMap {
		var cost int64 = 1
		if m.calculateCost {
			cost = 0
		}
		m.cache.Set(k, v, cost)
	}
	if !m.allowAsyncWrites.Load() {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) MultiSetWithTTL(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	for k, v := range valMap {
		var cost int64 = 1
		if m.calculateCost {
			cost = 0
		}
		m.cache.SetWithTTL(k, v, cost, expiration)
	}
	if !m.allowAsyncWrites.Load() {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	val, found := m.cache.Get(key)
	return val, found, nil
}

// GetDel holds getDel across the read and the delete so an entry is returned
// as found to at most one caller, matching the atomic GETDEL (redis) and
// single-transaction (badgerdb, mcache) behaviour of the other drivers.
// ristretto has no atomic get-and-delete, but its Del removes the entry from
// the store before returning, so the pair is enough. Only GetDel takes this
// lock; Get/Set stay lock-free.
func (m *Cache[T]) GetDel(ctx context.Context, key string) (T, bool, error) {
	m.getDel.Lock()
	defer m.getDel.Unlock()

	val, found := m.cache.Get(key)
	if found {
		m.cache.Del(key)
	}
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

// Close releases resources owned by this Cache. When the ristretto cache was
// injected (not owned), Close is a no-op and leaves it open for its owner to
// close; only a cache created by this Cache is closed here.
func (m *Cache[T]) Close() error {
	if m.owned {
		m.cache.Close()
	}
	return nil
}

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
