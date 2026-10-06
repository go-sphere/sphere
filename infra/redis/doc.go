// Package redis builds a go-redis client from a URL.
//
// [NewClient] is the only entry point. It returns a plain *redis.Client from
// github.com/redis/go-redis/v9 that callers share with cache/redis, mq/redis,
// or other components; the caller closes it.
//
// # Usage
//
//	import redisconn "github.com/go-sphere/sphere/infra/redis"
//
//	client, err := redisconn.NewClient(redisconn.Config{URL: "redis://:password@localhost:6379/0"})
//	if err != nil {
//		return err // invalid URL
//	}
//	defer client.Close()
//
//	if err := client.Ping(ctx).Err(); err != nil {
//		return err // connectivity is checked here, not by NewClient
//	}
//
// NewClient only parses the URL; go-redis connects lazily on first use, so
// connectivity errors surface later, not at construction. It does not ping,
// pool-tune, or wrap sphere cache/mq types.
package redis
