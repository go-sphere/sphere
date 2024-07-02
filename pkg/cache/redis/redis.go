package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

type Config struct {
	Addr     string `json:"addr"`
	DB       int    `json:"db"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Cache struct {
	Client *redis.Client
}

func NewClient(conf *Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     conf.Addr,
		DB:       conf.DB,
		Username: conf.Username,
		Password: conf.Password,
	})
}

func NewRedisCache(client *redis.Client) *Cache {
	return &Cache{Client: client}
}

func (c *Cache) Set(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	return c.Client.Set(ctx, key, val, expiration).Err()
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.Client.Get(ctx, key).Bytes()
}

func (c *Cache) MultiSet(ctx context.Context, valMap map[string][]byte, expiration time.Duration) error {
	pipe := c.Client.Pipeline()
	for k, v := range valMap {
		pipe.Set(ctx, k, v, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) MultiGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	vals, err := c.Client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for i, key := range keys {
		if vals[i] != nil {
			result[key] = []byte(vals[i].(string))
		}
	}
	return result, nil
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	return c.Client.Del(ctx, keys...).Err()
}

func (c *Cache) DelAll(ctx context.Context) error {
	return c.Client.FlushAll(ctx).Err()
}

func (c *Cache) Close() error {
	return c.Client.Close()
}
