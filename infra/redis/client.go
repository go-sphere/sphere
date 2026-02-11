package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Config defines the configuration parameters for establishing a Redis connection.
type Config struct {
	URL string `json:"url"`
}

// NewClient creates and returns a new Redis client instance based on the provided configuration.
// It parses the Redis URL, establishes a connection, and verifies connectivity with a ping operation.
// Returns an error if the URL is invalid or the Redis server is unreachable.
func NewClient(conf Config) (*redis.Client, error) {
	options, err := redis.ParseURL(conf.URL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(options)
	err = client.Ping(context.Background()).Err()
	if err != nil {
		return nil, err
	}
	return client, nil
}
