package cache

import (
	"context"
	"io"
	"time"
)

// Core provides basic methods for single key-value operations in the cache.
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
type Bulk[S any] interface {
	// MultiSet stores multiple key-value pairs in the cache without expiration.
	MultiSet(ctx context.Context, valMap map[string]S) error
	// MultiGet retrieves multiple values from the cache by their keys, returning a map of found key-value pairs.
	MultiGet(ctx context.Context, keys []string) (map[string]S, error)
	// MultiDel removes multiple keys from the cache.
	MultiDel(ctx context.Context, keys []string) error
}

// TTL provides methods to set cache entries with a specified Time To Live (TTL).
type TTL[S any] interface {
	// SetWithTTL stores a key-value pair in the cache with a specified expiration duration.
	SetWithTTL(ctx context.Context, key string, val S, expiration time.Duration) error
	// MultiSetWithTTL stores multiple key-value pairs in the cache with a specified expiration duration.
	MultiSetWithTTL(ctx context.Context, valMap map[string]S, expiration time.Duration) error
}

// Evictor provides a method to clear all entries from the cache.
type Evictor interface {
	// DelAll removes all keys from the cache.
	DelAll(ctx context.Context) error
}

// ExpirableCache combines core cache operations with TTL functionality, allowing for expirable cache entries.
type ExpirableCache[S any] interface {
	Core[S]
	TTL[S]
}

// Cache defines a generic caching interface that provides CRUD operations for storing and retrieving typed values.
// The interface supports both simple operations and batch operations with optional TTL (Time To Live) functionality.
// S represents the type of values that can be stored in the cache.
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
