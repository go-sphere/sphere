// Package memory is the ristretto-backed in-process cache.Cache driver.
//
// High throughput, no KeyLister (NSCache.DelAll returns ErrNotSupported
// unless another lister is in the stack). Writes Wait() unless
// SetAllowAsyncWrites(true). Ristretto may drop writes under load; that is
// reported as success. Same-key mutations and GetDel use 128 FNV-striped
// mutexes. After Close, this is the only driver that returns cache.ErrClosed.
//
// NewMemoryCache and NewMemoryCacheWithCost own the ristretto instance and
// Close it. NewMemoryCacheWithRistretto does not. NewByteCache uses
// cost=len(bytes); NewMemoryCache[[]byte] uses cost=1 per item. ctx on CRUD
// methods is unused. UpdateMaxCost on a closed cache is a silent no-op.
package memory

import (
	"context"
	"slices"
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
	numGetDelShards    = 128
)

// Cache is the ristretto-backed cache.Cache. See the package comment for
// ownership, dropped writes, GetDel sharding, and ErrClosed.
type Cache[T any] struct {
	calculateCost    bool
	allowAsyncWrites atomic.Bool
	cache            *ristretto.Cache[string, T]
	// mutationShards serialize GetDel with writes and deletes of the same key,
	// while allowing mutations of keys in other shards to proceed in parallel.
	mutationShards [numGetDelShards]sync.Mutex
	// closeMu guards every call into ristretto against a concurrent Close.
	// ristretto's own Close sets its internal closed flag last, after it has
	// already closed setBuf and the stop channel, so its per-method guards do
	// not cover the teardown window: a concurrent Set parks forever on Wait
	// (nothing is left to release the wait sentinel) and a concurrent Del or
	// Clear panics with "send on closed channel". Operations take the read
	// lock, Close takes the write lock, so no call can be in flight while
	// ristretto tears down.
	closeMu sync.RWMutex
	closed  bool
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
// caller keeps ownership and must close the *ristretto.Cache itself. Close still
// marks this wrapper as closed, so further calls on the wrapper return
// cache.ErrClosed even though the injected cache stays usable by its owner.
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
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return
	}
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
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return cache.ErrClosed
	}
	shard := fnv32(key) % numGetDelShards
	m.mutationShards[shard].Lock()
	defer m.mutationShards[shard].Unlock()
	var cost int64 = 1
	if m.calculateCost {
		cost = 0
	}
	m.cache.Set(key, cloneValue(val), cost)
	if !m.allowAsyncWrites.Load() {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return cache.ErrClosed
	}
	shard := fnv32(key) % numGetDelShards
	m.mutationShards[shard].Lock()
	defer m.mutationShards[shard].Unlock()
	var cost int64 = 1
	if m.calculateCost {
		cost = 0
	}
	m.cache.SetWithTTL(key, cloneValue(val), cost, expiration)
	if !m.allowAsyncWrites.Load() {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) MultiSet(ctx context.Context, valMap map[string]T) error {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return cache.ErrClosed
	}
	keys := make([]string, 0, len(valMap))
	for key := range valMap {
		keys = append(keys, key)
	}
	unlock := m.lockMutationShards(keys)
	defer unlock()
	for k, v := range valMap {
		var cost int64 = 1
		if m.calculateCost {
			cost = 0
		}
		m.cache.Set(k, cloneValue(v), cost)
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
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return cache.ErrClosed
	}
	keys := make([]string, 0, len(valMap))
	for key := range valMap {
		keys = append(keys, key)
	}
	unlock := m.lockMutationShards(keys)
	defer unlock()
	for k, v := range valMap {
		var cost int64 = 1
		if m.calculateCost {
			cost = 0
		}
		m.cache.SetWithTTL(k, cloneValue(v), cost, expiration)
	}
	if !m.allowAsyncWrites.Load() {
		m.cache.Wait()
	}
	return nil
}

func (m *Cache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		var zero T
		return zero, false, cache.ErrClosed
	}
	val, found := m.cache.Get(key)
	return cloneValue(val), found, nil
}

// GetDel holds a striped per-shard lock across the read and the delete so an entry is
// returned as found to at most one caller, matching the atomic GETDEL (redis) and
// single-transaction (badgerdb, mcache) behaviour of the other drivers.
// ristretto has no atomic get-and-delete, but its Del removes the entry from
// the store before returning. Writes and deletes take the same key's shard, so
// GetDel cannot remove a value installed concurrently after its read.
func (m *Cache[T]) GetDel(ctx context.Context, key string) (T, bool, error) {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		var zero T
		return zero, false, cache.ErrClosed
	}

	shard := fnv32(key) % numGetDelShards
	m.mutationShards[shard].Lock()
	defer m.mutationShards[shard].Unlock()

	val, found := m.cache.Get(key)
	if found {
		m.cache.Del(key)
	}
	return cloneValue(val), found, nil
}

func fnv32(key string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return h
}

func (m *Cache[T]) lockMutationShards(keys []string) func() {
	seen := [numGetDelShards]bool{}
	shards := make([]int, 0, len(keys))
	for _, key := range keys {
		shard := int(fnv32(key) % numGetDelShards)
		if !seen[shard] {
			seen[shard] = true
			shards = append(shards, shard)
		}
	}
	slices.Sort(shards)
	for _, shard := range shards {
		m.mutationShards[shard].Lock()
	}
	return func() {
		for i := len(shards) - 1; i >= 0; i-- {
			m.mutationShards[shards[i]].Unlock()
		}
	}
}

func (m *Cache[T]) MultiGet(ctx context.Context, keys []string) (map[string]T, error) {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return nil, cache.ErrClosed
	}
	result := make(map[string]T)
	for _, key := range keys {
		val, found := m.cache.Get(key)
		if found {
			result[key] = cloneValue(val)
		}
	}
	return result, nil
}

func (m *Cache[T]) Del(ctx context.Context, key string) error {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return cache.ErrClosed
	}
	shard := fnv32(key) % numGetDelShards
	m.mutationShards[shard].Lock()
	defer m.mutationShards[shard].Unlock()
	m.cache.Del(key)
	return nil
}

func (m *Cache[T]) MultiDel(ctx context.Context, keys []string) error {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return cache.ErrClosed
	}
	unlock := m.lockMutationShards(keys)
	defer unlock()
	for _, key := range keys {
		m.cache.Del(key)
	}
	return nil
}

func (m *Cache[T]) DelAll(ctx context.Context) error {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return cache.ErrClosed
	}
	m.cache.Clear()
	return nil
}

func (m *Cache[T]) Exists(ctx context.Context, key string) (bool, error) {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return false, cache.ErrClosed
	}
	_, found := m.cache.Get(key)
	return found, nil
}

// Close releases resources owned by this Cache. When the ristretto cache was
// injected (not owned), the underlying cache is left open for its owner to
// close; only a cache created by this Cache is closed here. Close is idempotent
// and safe to call concurrently with any other method: it takes the write lock,
// so it waits for in-flight operations to finish and every later call returns
// cache.ErrClosed instead of reaching a torn-down ristretto.
func (m *Cache[T]) Close() error {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	if m.owned {
		m.cache.Close()
	}
	return nil
}

// Sync waits until buffered writes have been applied. After Close it returns
// cache.ErrClosed.
func (m *Cache[T]) Sync() error {
	m.closeMu.RLock()
	defer m.closeMu.RUnlock()
	if m.closed {
		return cache.ErrClosed
	}
	m.cache.Wait()
	return nil
}

// ByteCache is Cache[[]byte]. NewByteCache costs by len(bytes);
// NewMemoryCache[[]byte] costs 1 per item.
type ByteCache = Cache[[]byte]

// NewByteCache returns a ByteCache whose ristretto cost is len(bytes).
func NewByteCache() *ByteCache {
	return NewMemoryCacheWithCost[[]byte](func(bytes []byte) int64 {
		return int64(len(bytes))
	})
}

// cloneValue returns an independent copy of val when the value type is a byte
// slice, and val itself otherwise.
//
// ristretto stores the value as given and documents that appending to a cached
// slice may update the backing array behind the cache's back. Without a copy the
// cache would therefore share the caller's array in both directions: reusing an
// encoding buffer after Set rewrites what is cached, and appending to a value
// returned by Get writes into it in place whenever the slice has spare capacity.
// The redis and badgerdb drivers always return fresh slices, so skipping the
// copy here made the same code correct on one backend and silently corrupting on
// another.
//
// Only []byte is copied. Other value types are stored as-is, and the Core
// documentation states that contract.
func cloneValue[T any](val T) T {
	raw, ok := any(val).([]byte)
	if !ok || raw == nil {
		return val
	}
	if cloned, ok := any(slices.Clone(raw)).(T); ok {
		return cloned
	}
	return val
}
