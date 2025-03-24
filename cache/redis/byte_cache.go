package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrorType = fmt.Errorf("type error")

type ByteCache struct {
	Client *redis.Client
}

func NewByteCache(client *redis.Client) *ByteCache {
	return &ByteCache{Client: client}
}

func (c *ByteCache) Set(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	return c.Client.Set(ctx, key, val, expiration).Err()
}

func (c *ByteCache) Get(ctx context.Context, key string) (*[]byte, error) {
	val, err := c.Client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (c *ByteCache) Del(ctx context.Context, key string) error {
	return c.Client.Del(ctx, key).Err()
}

func (c *ByteCache) MultiSet(ctx context.Context, valMap map[string][]byte, expiration time.Duration) error {
	pipe := c.Client.Pipeline()
	for k, v := range valMap {
		pipe.Set(ctx, k, v, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *ByteCache) MultiGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	vals, err := c.Client.MGet(ctx, keys...).Result()
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

func (c *ByteCache) MultiDel(ctx context.Context, keys []string) error {
	return c.Client.Del(ctx, keys...).Err()
}

func (c *ByteCache) DelAll(ctx context.Context) error {
	return c.Client.FlushAll(ctx).Err()
}

func (c *ByteCache) Close() error {
	return c.Client.Close()
}
