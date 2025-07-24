package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	URL string `json:"url"`
}

func NewClient(conf *Config) (*redis.Client, error) {
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
