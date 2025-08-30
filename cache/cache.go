package cache

import (
	"context"
	"io"
	"time"
)

// Cache defines a generic caching interface that provides CRUD operations for storing and retrieving typed values.
// The interface supports both simple operations and batch operations with optional TTL (Time To Live) functionality.
// S represents the type of values that can be stored in the cache.
type Cache[S any] interface {
	// Set stores a key-value pair in the cache without expiration.
	Set(ctx context.Context, key string, val S) error
	// SetWithTTL stores a key-value pair in the cache with a specified expiration duration.
	SetWithTTL(ctx context.Context, key string, val S, expiration time.Duration) error
	// MultiSet stores multiple key-value pairs in the cache without expiration.
	MultiSet(ctx context.Context, valMap map[string]S) error
	// MultiSetWithTTL stores multiple key-value pairs in the cache with a specified expiration duration.
	MultiSetWithTTL(ctx context.Context, valMap map[string]S, expiration time.Duration) error

	// Get retrieves a value from the cache by key, returning the value, whether it was found, and any error.
	Get(ctx context.Context, key string) (S, bool, error)
	// MultiGet retrieves multiple values from the cache by their keys, returning a map of found key-value pairs.
	MultiGet(ctx context.Context, keys []string) (map[string]S, error)

	// Del removes a single key from the cache.
	Del(ctx context.Context, key string) error
	// MultiDel removes multiple keys from the cache.
	MultiDel(ctx context.Context, keys []string) error
	// DelAll removes all keys from the cache.
	DelAll(ctx context.Context) error

	// Exists checks whether a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	io.Closer
}

// ByteCache is a specialized cache for storing byte slices, commonly used for serialized data or raw content.
type ByteCache = Cache[[]byte]
