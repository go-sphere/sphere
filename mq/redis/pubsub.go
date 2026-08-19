package redis

import (
	"context"
	"errors"
	"sync"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/log"
	"github.com/redis/go-redis/v9"
)

// ErrPubSubClosed is returned by Subscribe after Close.
var ErrPubSubClosed = errors.New("redis mq: pubsub is closed")

// PubSub implements a Redis-backed publish-subscribe message system with typed message support.
// It uses Redis pub/sub functionality to broadcast messages to all subscribers.
type PubSub[T any] struct {
	client *redis.Client
	codec  codec.Codec

	subscriptions map[string][]*redis.PubSub
	// handlers tracks the consumer goroutines so Close and UnsubscribeAll can
	// wait for a handler that is mid-execution instead of returning while it is
	// still touching resources the caller is about to release.
	handlers sync.WaitGroup
	// closed marks the pubsub as terminated. Subscribe performs its network
	// round trip outside the lock, so without this flag a subscription
	// established while Close was running would be registered afterwards and
	// never closed by anyone.
	closed bool
	mu     sync.Mutex
}

// NewPubSub creates a new Redis-based publish-subscribe system with the specified options.
// A Redis client must be provided via WithClient option.
func NewPubSub[T any](opt ...Option) (*PubSub[T], error) {
	opts := newOptions(opt...)
	err := opts.validate()
	if err != nil {
		return nil, err
	}
	return &PubSub[T]{
		client:        opts.client,
		codec:         opts.codec,
		subscriptions: make(map[string][]*redis.PubSub),
	}, nil
}

func (p *PubSub[T]) Broadcast(ctx context.Context, topic string, data T) error {
	raw, err := p.codec.Marshal(data)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, topic, raw).Err()
}

func (p *PubSub[T]) Subscribe(ctx context.Context, topic string, handler func(data T) error) error {
	// The subscribe round trip happens outside the lock. Holding p.mu across it
	// let one unresponsive Redis connection block Close, UnsubscribeAll and every
	// other Subscribe for as long as the network took to give up — unbounded when
	// the client is configured without a read timeout — and serialised startup
	// into one round trip per topic.
	sub := p.client.Subscribe(ctx, topic)
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return err
	}

	p.mu.Lock()
	if p.closed {
		// Close ran while the subscribe round trip was in flight. Registering
		// now would leave a live connection nobody is going to close.
		p.mu.Unlock()
		_ = sub.Close()
		return ErrPubSubClosed
	}
	p.subscriptions[topic] = append(p.subscriptions[topic], sub)
	p.handlers.Add(1)
	p.mu.Unlock()

	go func() {
		defer p.handlers.Done()
		for msg := range sub.Channel() {
			var data T
			if err := p.codec.Unmarshal([]byte(msg.Payload), &data); err != nil {
				log.Error("failed to unmarshal subscription message", log.Err(err), log.String("topic", topic))
				continue
			}
			p.dispatch(topic, handler, data)
		}
	}()

	return nil
}

func (p *PubSub[T]) dispatch(topic string, handler func(data T) error, data T) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("recovered from panic in subscription handler", log.Any("error", r), log.String("topic", topic))
		}
	}()
	if err := handler(data); err != nil {
		log.Error("subscription handler error", log.Err(err), log.String("topic", topic))
	}
}

// UnsubscribeAll stops the topic's subscriptions and waits for handlers already
// running to return.
//
// The wait is not per-topic: go-redis reports a closed subscription only by
// closing its message channel, and the goroutines are tracked in one group, so
// this waits for every consumer currently draining. In practice
// UnsubscribeAll is used during teardown, where that is the intended effect.
func (p *PubSub[T]) UnsubscribeAll(ctx context.Context, topic string) error {
	p.mu.Lock()
	subs, ok := p.subscriptions[topic]
	if ok {
		delete(p.subscriptions, topic)
	}
	p.mu.Unlock()

	if !ok {
		return nil
	}
	var err error
	for _, sub := range subs {
		err = errors.Join(err, sub.Close())
	}
	// Waiting happens outside the lock: a handler is user code of unbounded
	// duration, and holding p.mu across it would block Broadcast and Subscribe
	// for as long as it runs.
	p.handlers.Wait()
	return err
}

// Close terminates every subscription and waits for handlers already running to
// return, so that once it returns no handler is still touching resources the
// caller is about to release. It is idempotent, and a Subscribe that completes
// afterwards returns ErrPubSubClosed rather than silently reviving the pubsub
// with a connection that nothing would ever close.
func (p *PubSub[T]) Close() error {
	p.mu.Lock()
	p.closed = true
	var err error
	for topic, subs := range p.subscriptions {
		for _, sub := range subs {
			err = errors.Join(err, sub.Close())
		}
		delete(p.subscriptions, topic)
	}
	p.mu.Unlock()

	p.handlers.Wait()
	return err
}
