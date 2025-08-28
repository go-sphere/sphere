package redis

import (
	"errors"

	"github.com/go-sphere/sphere/core/codec"
	"github.com/redis/go-redis/v9"
)

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

type Option func(*options)

func WithClient(client *redis.Client) Option {
	return func(o *options) {
		o.client = client
	}
}

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
