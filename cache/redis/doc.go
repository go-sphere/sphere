// Package redis is the Redis-backed cache.ByteCache driver (go-redis).
//
// Use [NewByteCache] with a shared *redis.Client (for example one built by
// infra/redis.NewClient and also used by mq), or [NewByteCacheWithOptions] to
// let the cache own its client. Wrap it with cache.NewJsonCache for typed
// values and nscache to scope keys.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/cache/redis"
//		redisconn "github.com/go-sphere/sphere/infra/redis"
//	)
//
//	client, err := redisconn.NewClient(redisconn.Config{URL: "redis://localhost:6379/0"})
//	if err != nil {
//		return err
//	}
//	defer client.Close() // NewByteCache does not close an injected client
//
//	c := redis.NewByteCache(client)
//	if err := c.SetWithTTL(ctx, "k", []byte("v"), time.Minute); err != nil {
//		return err
//	}
//	v, found, err := c.Get(ctx, "k")
//
// DelAll is FlushDB of the selected database, not FLUSHALL and not "this
// wrapper's keys". Do not share that DB with mq keys if you call DelAll.
// Keys uses SCAN with a glob-escaped MATCH prefix* of the selected DB.
// Empty MultiGet/MultiDel short-circuit because Redis rejects 0-arg MGET/DEL.
package redis
