package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrorType = fmt.Errorf("type error")

// ByteCache is a Redis-backed cache implementation for storing raw byte data.
// It provides direct access to Redis operations without any encoding/decoding overhead.
type ByteCache struct {
	client *redis.Client
}

// NewByteCache creates a new Redis byte cache using the provided Redis client.
func NewByteCache(client *redis.Client) *ByteCache {
	return &ByteCache{client: client}
}

func (c *ByteCache) Set(ctx context.Context, key string, val []byte) error {
	return c.SetWithTTL(ctx, key, val, redis.KeepTTL)
}

func (c *ByteCache) SetWithTTL(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	return c.client.Set(ctx, key, val, expiration).Err()
}

func (c *ByteCache) MultiSet(ctx context.Context, valMap map[string][]byte) error {
	return c.MultiSetWithTTL(ctx, valMap, redis.KeepTTL)
}

func (c *ByteCache) MultiSetWithTTL(ctx context.Context, valMap map[string][]byte, expiration time.Duration) error {
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

func (c *ByteCache) MultiGet(ctx context.Context, keys []string) (map[string][]byte, error) {
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
	return c.client.Del(ctx, keys...).Err()
}

func (c *ByteCache) DelAll(ctx context.Context) error {
	return c.client.FlushAll(ctx).Err()
}

func (c *ByteCache) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (c *ByteCache) Close() error {
	return c.client.Close()
}
