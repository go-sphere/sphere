// Package redis is the Redis-backed cache.ByteCache driver (go-redis).
//
// DelAll is FlushDB of the selected database, not FLUSHALL and not "this
// wrapper's keys". Do not share that DB with mq keys if you call DelAll.
// Keys uses SCAN with a glob-escaped MATCH prefix* of the selected DB.
// NewByteCache(client) does not close the client; NewByteCacheWithOptions
// does. Empty MultiGet/MultiDel short-circuit because Redis rejects 0-arg
// MGET/DEL.
package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sphere/sphere/cache"
	"github.com/redis/go-redis/v9"
)

const scanBatchSize = 256

// ErrorType is returned by MultiGet when an MGET value is not a string.
var ErrorType = fmt.Errorf("type error")

// ByteCache is a Redis-backed cache implementation for storing raw byte data.
// It provides direct access to Redis operations without any encoding/decoding overhead.
type ByteCache struct {
	client *redis.Client
	// owned reports whether this cache created the underlying client and is
	// therefore responsible for closing it. Injected clients are not owned.
	owned bool
}

// NewByteCache creates a new Redis byte cache using the provided Redis client.
// The client is injected, not owned: it is not created here, so Close does not
// close it. The caller keeps ownership and must close the client itself (this
// lets a single client be shared with, e.g., an mq driver).
func NewByteCache(client *redis.Client) *ByteCache {
	return &ByteCache{client: client, owned: false}
}

// NewByteCacheWithOptions creates a new Redis byte cache from connection options,
// building the client here. The client is owned by this cache, so Close closes it.
// Use NewByteCache instead when the client is shared with other components.
func NewByteCacheWithOptions(opts *redis.Options) *ByteCache {
	return &ByteCache{client: redis.NewClient(opts), owned: true}
}

func (c *ByteCache) Set(ctx context.Context, key string, val []byte) error {
	// expiration 0 issues SET without EX/PX, which clears any existing TTL so
	// the key never expires (per the TTL contract in the cache package).
	return c.SetWithTTL(ctx, key, val, 0)
}

func (c *ByteCache) SetWithTTL(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	return c.client.Set(ctx, key, val, expiration).Err()
}

func (c *ByteCache) MultiSet(ctx context.Context, valMap map[string][]byte) error {
	return c.MultiSetWithTTL(ctx, valMap, 0)
}

func (c *ByteCache) MultiSetWithTTL(ctx context.Context, valMap map[string][]byte, expiration time.Duration) error {
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	pipe := c.client.Pipeline()
	for k, v := range valMap {
		pipe.Set(ctx, k, v, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *ByteCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return val, true, nil
}

func (c *ByteCache) GetDel(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := c.client.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return val, true, nil
}

func (c *ByteCache) MultiGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	// Redis MGET rejects a zero-argument call ("wrong number of arguments"),
	// while the other drivers return an empty result for empty input. Return
	// early to keep behavior consistent and avoid the protocol error.
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}
	vals, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for i, key := range keys {
		if vals[i] != nil {
			raw, ok := vals[i].(string)
			if !ok {
				return nil, ErrorType
			}
			result[key] = []byte(raw)
		}
	}
	return result, nil
}

func (c *ByteCache) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *ByteCache) MultiDel(ctx context.Context, keys []string) error {
	// Redis DEL rejects a zero-argument call ("wrong number of arguments"),
	// while the other drivers treat an empty key set as a no-op. Return early
	// to keep behavior consistent and avoid the protocol error.
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

// DelAll runs FlushDB on the selected database. It is not FLUSHALL and does
// not limit itself to keys written through this wrapper.
func (c *ByteCache) DelAll(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}

func (c *ByteCache) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// Keys returns every key in the selected database whose name starts with prefix.
// It drains SCAN with a glob-escaped MATCH pattern so metacharacters in the
// prefix (*, ?, [, ], \) are treated literally. If any SCAN batch fails the
// partial result is discarded and the error is returned.
func (c *ByteCache) Keys(ctx context.Context, prefix string) ([]string, error) {
	pattern := escapeGlob(prefix) + "*"
	var (
		cursor uint64
		keys   []string
	)
	for {
		batch, next, err := c.client.Scan(ctx, cursor, pattern, scanBatchSize).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			return keys, nil
		}
	}
}

// escapeGlob quotes Redis glob metacharacters so the input matches literally.
// See https://redis.io/commands/keys for the supported pattern syntax.
func escapeGlob(s string) string {
	if !strings.ContainsAny(s, `*?[]\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		switch r {
		case '*', '?', '[', ']', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Close releases resources owned by this cache. When the Redis client was
// injected (not owned), Close is a no-op and leaves the client open for its
// owner to close; only a client created by this cache is closed here.
func (c *ByteCache) Close() error {
	if c.owned {
		return c.client.Close()
	}
	return nil
}
