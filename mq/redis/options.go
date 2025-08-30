package redis

import (
	"errors"

	"github.com/go-sphere/sphere/core/codec"
	"github.com/redis/go-redis/v9"
)

// options holds configuration parameters for Redis-based message queue implementations.
type options struct {
	client *redis.Client
	codec  codec.Codec
}

func newOptions(opt ...Option) *options {
	opts := &options{
		codec: codec.JsonCodec(),
	}
	for _, o := range opt {
		o(opts)
	}
	return opts
}

// Option defines a function type for configuring Redis message queue options.
type Option func(*options)

// WithClient sets the Redis client instance to be used for message queue operations.
// This option is required and must be provided when creating Redis-based message queues.
func WithClient(client *redis.Client) Option {
	return func(o *options) {
		o.client = client
	}
}

// WithCodec sets the codec used for message serialization and deserialization.
// If not specified, JSON codec is used by default.
func WithCodec(codec codec.Codec) Option {
	return func(o *options) {
		o.codec = codec
	}
}

func (o *options) validate() error {
	if o.client == nil {
		return errors.New("redis client is required")
	}
	if o.codec == nil {
		return errors.New("codec is required")
	}
	return nil
}
