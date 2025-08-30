package redis

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/core/codec"
	"github.com/redis/go-redis/v9"
)

// Cache is a Redis-backed cache implementation that stores typed values by encoding them with a codec.
// It wraps a ByteCache and handles serialization/deserialization automatically.
type Cache[T any] struct {
	cache *ByteCache
	codec codec.Codec
}

// NewCache creates a new Redis cache with the specified codec for value serialization.
// The codec handles the conversion between typed values and byte slices for Redis storage.
func NewCache[T any](client *redis.Client, codec codec.Codec) *Cache[T] {
	return &Cache[T]{
		cache: NewByteCache(client),
		codec: codec,
	}
}

func (m *Cache[T]) Set(ctx context.Context, key string, val T) error {
	raw, err := m.codec.Marshal(val)
	if err != nil {
		return err
	}
	return m.cache.Set(ctx, key, raw)
}

func (m *Cache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	raw, err := m.codec.Marshal(val)
	if err != nil {
		return err
	}
	return m.cache.SetWithTTL(ctx, key, raw, expiration)
}

func (m *Cache[T]) MultiSet(ctx context.Context, valMap map[string]T) error {
	rawMap := make(map[string][]byte, len(valMap))
	for k, v := range valMap {
		raw, err := m.codec.Marshal(v)
		if err != nil {
			return err
		}
		rawMap[k] = raw
	}
	return m.cache.MultiSet(ctx, rawMap)
}

func (m *Cache[T]) MultiSetWithTTL(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	rawMap := make(map[string][]byte, len(valMap))
	for k, v := range valMap {
		raw, err := m.codec.Marshal(v)
		if err != nil {
			return err
		}
		rawMap[k] = raw
	}
	return m.cache.MultiSetWithTTL(ctx, rawMap, expiration)
}

func (m *Cache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	raw, found, err := m.cache.Get(ctx, key)
	var val T
	if err != nil {
		return val, false, err
	}
	if !found {
		return val, false, nil
	}
	err = m.codec.Unmarshal(raw, &val)
	if err != nil {
		return val, false, err
	}
	return val, true, nil
}

func (m *Cache[T]) MultiGet(ctx context.Context, keys []string) (map[string]T, error) {
	rawMap, err := m.cache.MultiGet(ctx, keys)
	if err != nil {
		return nil, err
	}
	result := make(map[string]T)
	for _, key := range keys {
		raw, ok := rawMap[key]
		if !ok {
			continue
		}
		var val T
		err = m.codec.Unmarshal(raw, &val)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (m *Cache[T]) Del(ctx context.Context, key string) error {
	return m.cache.Del(ctx, key)
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
	return m.cache.DelAll(ctx)
}

func (m *Cache[T]) Exists(ctx context.Context, key string) (bool, error) {
	return m.cache.Exists(ctx, key)
}

func (m *Cache[T]) Close() error {
	return nil
}
