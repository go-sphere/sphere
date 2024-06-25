package cache

import (
	"github.com/coocood/freecache"
)

type memoryCache struct {
	cache *freecache.Cache
}

func NewMemoryCache() Cache {
	return &memoryCache{
		cache: freecache.NewCache(1024 * 1024 * 1024),
	}
}

func (c *memoryCache) Set(key string, value []byte, expireSeconds int) error {
	return c.cache.Set([]byte(key), value, expireSeconds)
}

func (c *memoryCache) Delete(key string) error {
	_ = c.cache.Del([]byte(key))
	return nil
}

func (c *memoryCache) Get(key string) ([]byte, error) {
	return c.cache.Get([]byte(key))
}

func (c *memoryCache) Reset() error {
	c.cache.Clear()
	return nil
}
