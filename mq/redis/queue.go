package redis

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sphere/confstore/codec"
	"github.com/redis/go-redis/v9"
)

var errInvalidBLPopResponse = errors.New("redis mq: invalid BLPOP response")

// Queue implements a Redis-backed point-to-point message queue with typed message support.
// It uses Redis lists to provide FIFO message delivery semantics.
type Queue[T any] struct {
	client *redis.Client
	codec  codec.Codec
}

// NewQueue creates a new Redis-based queue with the specified options.
// A Redis client must be provided via WithClient option.
func NewQueue[T any](opt ...Option) (*Queue[T], error) {
	opts := newOptions(opt...)
	err := opts.validate()
	if err != nil {
		return nil, err
	}
	return &Queue[T]{
		client: opts.client,
		codec:  opts.codec,
	}, nil
}

func (q *Queue[T]) Publish(ctx context.Context, topic string, data T) error {
	raw, err := q.codec.Marshal(data)
	if err != nil {
		return err
	}
	return q.client.RPush(ctx, topic, raw).Err()
}

func (q *Queue[T]) Consume(ctx context.Context, topic string) (T, error) {
	var zero T
	resp, err := q.client.BLPop(ctx, 0, topic).Result()
	if err != nil {
		return zero, err
	}
	if len(resp) < 2 {
		return zero, fmt.Errorf("%w: %v", errInvalidBLPopResponse, resp)
	}
	raw := []byte(resp[1])
	var data T
	err = q.codec.Unmarshal(raw, &data)
	if err != nil {
		return zero, err
	}
	return data, nil
}

func (q *Queue[T]) TryConsume(ctx context.Context, topic string) (T, bool, error) {
	var zero T
	raw, err := q.client.LPop(ctx, topic).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return zero, false, nil
		}
		return zero, false, err
	}
	var data T
	err = q.codec.Unmarshal(raw, &data)
	if err != nil {
		return zero, false, err
	}
	return data, true, nil
}

func (q *Queue[T]) PurgeQueue(ctx context.Context, topic string) error {
	return q.client.Del(ctx, topic).Err()
}

func (q *Queue[T]) Close() error {
	return q.client.Close()
}
