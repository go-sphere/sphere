package redis

import (
	"context"
	"errors"
	"sync"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/log"
	"github.com/redis/go-redis/v9"
)

// PubSub implements a Redis-backed publish-subscribe message system with typed message support.
// It uses Redis pub/sub functionality to broadcast messages to all subscribers.
type PubSub[T any] struct {
	client *redis.Client
	codec  codec.Codec

	subscriptions map[string][]*redis.PubSub
	mu            sync.Mutex
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
	p.mu.Lock()
	defer p.mu.Unlock()

	sub := p.client.Subscribe(ctx, topic)
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return err
	}
	p.subscriptions[topic] = append(p.subscriptions[topic], sub)

	go func() {
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

func (p *PubSub[T]) UnsubscribeAll(ctx context.Context, topic string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if subs, ok := p.subscriptions[topic]; ok {
		delete(p.subscriptions, topic)
		var err error
		for _, sub := range subs {
			err = errors.Join(err, sub.Close())
		}
		return err
	}

	return nil
}

func (p *PubSub[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var err error
	for topic, subs := range p.subscriptions {
		for _, sub := range subs {
			err = errors.Join(err, sub.Close())
		}
		delete(p.subscriptions, topic)
	}

	return err
}
