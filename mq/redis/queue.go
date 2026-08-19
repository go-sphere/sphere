package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/redis/go-redis/v9"
)

var errInvalidBLPopResponse = errors.New("redis mq: invalid BLPOP response")

// DecodeError reports a message that was received but could not be decoded.
//
// The element is already gone from the list by the time decoding runs — Redis
// has no way to pop conditionally — so the raw bytes are carried here rather
// than discarded. Without them the payload is unrecoverable: it is neither in
// the queue nor in the error. A queue can hold undecodable entries whenever
// something else writes to the same key, the codec changes, or two differently
// typed queues share a topic.
type DecodeError struct {
	Topic string
	Raw   []byte
	Err   error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("redis mq: decode message from %q: %v", e.Topic, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }

// blockingConsumePoll bounds a single BLPOP so Consume can observe context
// cancellation. BLPOP with a zero timeout blocks on the socket forever: go-redis
// sets no read deadline for it, and unless the client was built with
// ContextTimeoutEnabled it passes context.Background() down to the connection,
// so cancelling the caller's context has no effect on a call already in flight.
// That is exactly the shutdown path — a consumer parked on an empty queue —
// which would otherwise leak its goroutine for the life of the process. Each
// BLPOP therefore blocks for at most this long and Consume re-checks the
// context between rounds.
//
// Cancellation is observed within one poll rather than immediately. One second
// is the floor: go-redis sends the BLPOP timeout in whole seconds and rounds
// anything shorter up to 1s. Abandoning an in-flight BLPOP instead would return
// sooner but drop the element it had already popped, so waiting it out is what
// keeps a shutdown from eating a message.
const blockingConsumePoll = time.Second

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
	for {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		resp, err := q.client.BLPop(ctx, blockingConsumePoll, topic).Result()
		if err != nil {
			// A timed-out BLPOP is not a failure, it is this call's way of
			// yielding so the context can be checked.
			if errors.Is(err, redis.Nil) {
				continue
			}
			return zero, err
		}
		if len(resp) < 2 {
			return zero, fmt.Errorf("%w: %v", errInvalidBLPopResponse, resp)
		}
		raw := []byte(resp[1])
		var data T
		if err = q.codec.Unmarshal(raw, &data); err != nil {
			return zero, &DecodeError{Topic: topic, Raw: raw, Err: err}
		}
		return data, nil
	}
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
	if err = q.codec.Unmarshal(raw, &data); err != nil {
		// found is true: a message really was taken off the queue. Reporting
		// false here said "nothing was waiting" while the element had already
		// been consumed and destroyed.
		return zero, true, &DecodeError{Topic: topic, Raw: raw, Err: err}
	}
	return data, true, nil
}

func (q *Queue[T]) PurgeQueue(ctx context.Context, topic string) error {
	return q.client.Del(ctx, topic).Err()
}

func (q *Queue[T]) Close() error {
	return nil
}
