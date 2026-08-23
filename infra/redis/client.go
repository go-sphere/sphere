// Package redis builds a go-redis client from a URL.
//
// NewClient only parses the URL; go-redis connects lazily on first use, so
// connectivity errors surface later, not at construction. It does not ping,
// pool-tune, or wrap sphere cache/mq types — it returns *redis.Client.
package redis

import (
	"github.com/redis/go-redis/v9"
)

// Config defines the configuration parameters for establishing a Redis connection.
type Config struct {
	URL string `json:"url" yaml:"url"`
}

// NewClient creates and returns a new Redis client instance based on the provided configuration.
// It only parses the Redis URL and builds the client; go-redis connects lazily on first use, so
// connectivity errors surface when the client is actually used rather than at construction time.
// Returns an error if the URL is invalid.
func NewClient(conf Config) (*redis.Client, error) {
	options, err := redis.ParseURL(conf.URL)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(options), nil
}
