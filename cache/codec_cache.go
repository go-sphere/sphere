package cache

import (
	"context"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/log"
)

var _ Cache[any] = (*CodecCache[any])(nil)

// CodecCache adapts a ByteCache to a typed Cache[T] using the provided codec.
// Close is a no-op; DelAll forwards to the inner ByteCache (redis FlushDB if
// that is the backend). MultiGet omits undecodable entries without deleting
// them. Get returns a decode error wrapping ErrDecode without deleting; GetDel atomically
// consumes the raw entry before decoding it and reports found=true when that
// entry is undecodable.
type CodecCache[T any] struct {
	cache ByteCache
	codec codec.Codec
}

// NewCodecCache creates a typed cache adapter from any ByteCache and codec.
//
// CodecCache adds no synchronization of its own, so the codec must be safe for
// concurrent use: every adapter method may call Marshal/Unmarshal from several
// goroutines, and one codec can be shared by several adapters.
func NewCodecCache[T any](cache ByteCache, codec codec.Codec) *CodecCache[T] {
	return &CodecCache[T]{
		cache: cache,
		codec: codec,
	}
}

// NewJsonCache creates a typed cache adapter using JSON encoding.
func NewJsonCache[T any](cache ByteCache) *CodecCache[T] {
	return NewCodecCache[T](cache, codec.JsonCodec())
}

// GetByteCache returns the underlying byte cache.
func (m *CodecCache[T]) GetByteCache() ByteCache {
	return m.cache
}

// GetCodec returns the codec used for serialization.
func (m *CodecCache[T]) GetCodec() codec.Codec {
	return m.codec
}

// Set encodes val with the codec and stores it without expiration. An encoding
// error is returned and nothing is written.
func (m *CodecCache[T]) Set(ctx context.Context, key string, val T) error {
	raw, err := m.codec.Marshal(val)
	if err != nil {
		return err
	}
	return m.cache.Set(ctx, key, raw)
}

// SetWithTTL encodes val and stores it with the cache.TTL expiration rules.
// An encoding error is returned and nothing is written.
func (m *CodecCache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	raw, err := m.codec.Marshal(val)
	if err != nil {
		return err
	}
	return m.cache.SetWithTTL(ctx, key, raw, expiration)
}

// MultiSet encodes every value and stores them without expiration. If any
// value fails to encode, nothing is written.
func (m *CodecCache[T]) MultiSet(ctx context.Context, valMap map[string]T) error {
	rawMap, err := m.marshalMap(valMap)
	if err != nil {
		return err
	}
	return m.cache.MultiSet(ctx, rawMap)
}

// MultiSetWithTTL encodes every value and stores them with expiration. If
// any value fails to encode, nothing is written.
func (m *CodecCache[T]) MultiSetWithTTL(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	rawMap, err := m.marshalMap(valMap)
	if err != nil {
		return err
	}
	return m.cache.MultiSetWithTTL(ctx, rawMap, expiration)
}

func (m *CodecCache[T]) marshalMap(valMap map[string]T) (map[string][]byte, error) {
	rawMap := make(map[string][]byte, len(valMap))
	for k, v := range valMap {
		raw, err := m.codec.Marshal(v)
		if err != nil {
			return nil, err
		}
		rawMap[k] = raw
	}
	return rawMap, nil
}

// Get loads and decodes key. A miss returns (zero, false, nil). A decode
// error returns found=false with an error wrapping ErrDecode and leaves the
// entry in place.
func (m *CodecCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
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
		var zero T
		return zero, false, decodeError(err)
	}
	return val, true, nil
}

// GetDel atomically consumes key from the byte cache, then decodes it. If the
// consumed entry cannot be decoded, GetDel returns found=true with the decode
// error; the entry is already deleted.
func (m *CodecCache[T]) GetDel(ctx context.Context, key string) (T, bool, error) {
	raw, found, err := m.cache.GetDel(ctx, key)
	var val T
	if err != nil {
		return val, false, err
	}
	if !found {
		return val, false, nil
	}
	err = m.codec.Unmarshal(raw, &val)
	if err != nil {
		return val, true, err
	}
	return val, true, nil
}

// MultiGet retrieves multiple values by key. Entries the codec cannot decode
// are skipped with a warning instead of failing the batch, so a nil-error
// result may omit keys that exist in the backend.
func (m *CodecCache[T]) MultiGet(ctx context.Context, keys []string) (map[string]T, error) {
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
		if err = m.codec.Unmarshal(raw, &val); err != nil {
			// Skip an undecodable entry (older schema, or another type sharing the
			// key space) instead of failing the whole batch. It is not deleted:
			// another caller may have repaired the key since the snapshot, and
			// single-key Get leaves such entries in place too.
			log.Warn("cache: skipping undecodable entry",
				log.String("key", key),
				log.Err(err),
			)
			continue
		}
		result[key] = val
	}
	return result, nil
}

// Del forwards to the underlying byte cache.
func (m *CodecCache[T]) Del(ctx context.Context, key string) error {
	return m.cache.Del(ctx, key)
}

// MultiDel forwards to the underlying byte cache.
func (m *CodecCache[T]) MultiDel(ctx context.Context, keys []string) error {
	return m.cache.MultiDel(ctx, keys)
}

// DelAll forwards to the underlying byte cache, so its blast radius is the
// backend's (redis FlushDB, the whole process cache for memory/mcache).
func (m *CodecCache[T]) DelAll(ctx context.Context) error {
	return m.cache.DelAll(ctx)
}

// Keys forwards prefix listing to the underlying byte cache when it supports
// KeyLister, so callers that wrap a typed adapter in NSCache (or otherwise
// rely on the optional listing capability) keep working. Returns
// ErrNotSupported when the underlying byte cache does not implement
// KeyLister.
func (m *CodecCache[T]) Keys(ctx context.Context, prefix string) ([]string, error) {
	lister, ok := m.cache.(KeyLister)
	if !ok {
		return nil, ErrNotSupported
	}
	return lister.Keys(ctx, prefix)
}

// Exists forwards to the underlying byte cache without decoding the value.
func (m *CodecCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	return m.cache.Exists(ctx, key)
}

// Close is a no-op. The byte cache is injected, not created here, so this
// adapter never owns it and closing the adapter must not take the backend
// down with it — that is what lets one byte cache serve several typed
// adapters. The caller keeps ownership and closes the backend itself, the
// same rule the driver constructors follow.
func (m *CodecCache[T]) Close() error {
	return nil
}
