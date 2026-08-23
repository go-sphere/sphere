// Package cache is the typed key/value contract used by Sphere services, plus
// adapters that sit on top of any driver.
//
// Cache[S] combines Core, Bulk, TTL, Evictor, and io.Closer. Drivers live in
// subpackages: memory (ristretto), mcache (plain map), redis, badgerdb
// (persistent []byte), and nocache (always-miss). nscache.NSCache prefixes
// keys so several logical caches can share one backend.
//
// # Capabilities
//
//   - Single-key CRUD: Set, Get, GetDel, Del, Exists.
//   - Batches: MultiSet, MultiGet, MultiDel. Batches are not atomic.
//   - TTL: SetWithTTL / MultiSetWithTTL. expiration > 0 expires; 0 never
//     expires and clears any existing TTL; < 0 returns ErrInvalidTTL and
//     writes nothing.
//   - Optional KeyLister.Keys: implemented by mcache, badgerdb, redis,
//     nocache, CodecCache, and NSCache. The ristretto-backed memory driver
//     does not implement it. NSCache.DelAll needs a KeyLister; without one
//     it returns ErrNotSupported.
//   - CodecCache / NewJsonCache: ByteCache → Cache[T] via confstore/codec.
//     Close on a wrapper is a no-op; the caller closes the backend.
//   - Loader helpers (Set, GetEx, GetJsonEx, …): read-through fill, JSON
//     encoding, optional singleflight, dynamic TTL.
//
// # Ownership
//
// A constructor that opens a resource closes it; a constructor that receives
// one does not. Wrappers never close the injected backend, so one ByteCache
// can back several CodecCache / NSCache instances.
//
// # DelAll blast radius
//
// redis.DelAll is FlushDB of the selected Redis database, not this wrapper's
// keys. memory and mcache clear the whole process cache. NSCache deletes only
// its namespace. Do not share a Redis DB with mq keys if you call DelAll on
// the cache.
//
// ErrClosed is returned after Close only by the memory driver. mcache and
// nocache Close are no-ops and later operations still succeed. redis and
// badgerdb surface their own closed-connection error when the owned client
// or DB was actually closed.
package cache

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotSupported is returned by optional operations that a given driver does
// not implement. For example, NSCache.DelAll returns it when the wrapped
// cache does not implement KeyLister.
var ErrNotSupported = errors.New("cache: operation not supported")

// ErrInvalidTTL is returned by SetWithTTL/MultiSetWithTTL when the caller
// passes a negative expiration. A negative TTL has no consistent meaning
// across drivers, so it is rejected instead of being silently stored or
// dropped. Use a zero expiration for entries that never expire.
var ErrInvalidTTL = errors.New("cache: negative TTL is invalid")

// ErrClosed is returned by operations invoked after Close on drivers that
// honor it (currently the ristretto-backed memory driver). Close may run
// concurrently with any other method — a task group cancels its members
// concurrently, so an in-flight request can still be writing while the cache is
// torn down — and those drivers must report that as an error rather than
// deadlocking or panicking. redis and badgerdb surface their own
// closed-connection error when the owned client or DB was closed. mcache and
// nocache Close are no-ops.
var ErrClosed = errors.New("cache: cache is closed")

// KeyLister is an optional capability that returns every key whose name
// starts with prefix (pass "" to list all keys). Drivers that can scan
// their keyspace cheaply implement this: mcache, badgerdb, redis, and
// nocache (always empty). The ristretto-backed memory driver does not.
//
// The wrappers (CodecCache and nscache.NSCache) implement it too, forwarding
// to the cache they wrap and returning ErrNotSupported when that cache has no
// KeyLister. That is what keeps NSCache.DelAll working through a layer of
// wrapping instead of degrading to ErrNotSupported.
//
// Implementations may load the full key set into the returned slice and
// must honour ctx cancellation.
type KeyLister interface {
	Keys(ctx context.Context, prefix string) ([]string, error)
}

// Core provides basic methods for single key-value operations in the cache.
//
// Value ownership: for byte-slice caches (ByteCache and the []byte instantiation
// of any driver), stored and returned values are independent copies. A caller
// may reuse an encoding buffer after Set and may append to a value returned by
// Get without disturbing what is cached.
//
// For every other value type the cache stores whatever it is given: a driver
// backed by an in-process map keeps the caller's own value, so mutating a value
// after Set — or mutating one returned by Get — mutates the cache. Cache values
// of other types should be treated as immutable once handed over.
type Core[S any] interface {
	// Set stores a key-value pair in the cache without expiration.
	Set(ctx context.Context, key string, val S) error
	// Get retrieves a value from the cache by key, returning the value, whether it was found, and any error.
	Get(ctx context.Context, key string) (S, bool, error)
	// GetDel retrieves a value from the cache by key and deletes the key, returning the value, whether it was found, and any error.
	GetDel(ctx context.Context, key string) (S, bool, error)
	// Del removes a single key from the cache.
	Del(ctx context.Context, key string) error
	// Exists checks whether a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)
}

// Bulk provides methods for batch operations on multiple key-value pairs.
//
// Batch operations are not atomic. A non-nil error means at least one entry
// failed; entries that succeeded before it may already be visible and are not
// rolled back, so callers must not assume the cache is unchanged after a
// failure. Compensating for a failed write by deleting what it "would have"
// written is therefore unsafe — the batch may be half applied.
//
// Some drivers do apply a batch atomically (badgerdb writes one transaction,
// mcache holds a single lock), but that is an implementation detail: redis
// pipelines the commands and cannot roll back the ones already sent, and the
// ristretto-backed memory driver may drop an individual write under load with
// no way to undo the rest.
type Bulk[S any] interface {
	// MultiSet stores multiple key-value pairs in the cache without expiration.
	MultiSet(ctx context.Context, valMap map[string]S) error
	// MultiGet retrieves multiple values from the cache by their keys, returning a map of found key-value pairs.
	MultiGet(ctx context.Context, keys []string) (map[string]S, error)
	// MultiDel removes multiple keys from the cache.
	MultiDel(ctx context.Context, keys []string) error
}

// TTL provides methods to set cache entries with a specified Time To Live (TTL).
//
// The expiration argument shares the same semantics across all drivers:
//   - expiration > 0: the entry expires after the given duration.
//   - expiration == 0: the entry never expires; it persists until an explicit
//     Del (any existing TTL on the key is cleared).
//   - expiration < 0: invalid; the call returns ErrInvalidTTL and writes
//     nothing.
type TTL[S any] interface {
	// SetWithTTL stores a key-value pair in the cache with a specified expiration duration.
	SetWithTTL(ctx context.Context, key string, val S, expiration time.Duration) error
	// MultiSetWithTTL stores multiple key-value pairs in the cache with a specified expiration duration.
	MultiSetWithTTL(ctx context.Context, valMap map[string]S, expiration time.Duration) error
}

// Evictor clears the cache. DelAll's blast radius is driver-dependent:
// redis FlushDB wipes the selected Redis database, memory/mcache clear the
// whole process cache, NSCache deletes only its namespace.
type Evictor interface {
	// DelAll removes keys according to the driver's blast-radius rules.
	DelAll(ctx context.Context) error
}

// ExpirableCache is Core plus TTL, without Bulk, Evictor, or Close. Loader
// helpers (Set, GetEx) accept this narrower surface.
type ExpirableCache[S any] interface {
	Core[S]
	TTL[S]
}

// Cache is the full driver contract: Core, Bulk, TTL, Evictor, and io.Closer.
type Cache[S any] interface {
	Core[S]
	Bulk[S]
	TTL[S]
	Evictor
	io.Closer
}

// ByteCache is a specialized cache for storing byte slices, commonly used for serialized data or raw content.
type ByteCache = Cache[[]byte]

// ExpirableByteCache is a specialized expirable cache for storing byte slices with TTL support.
type ExpirableByteCache = ExpirableCache[[]byte]
