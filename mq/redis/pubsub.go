package redis

import (
	"context"
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

	subscriptions map[string]*redis.PubSub
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
		subscriptions: make(map[string]*redis.PubSub),
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
	p.subscriptions[topic] = sub

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("recovered from panic in subscription handler", log.Any("error", r))
			}
		}()
		for msg := range sub.Channel() {
			var data T
			err := p.codec.Unmarshal([]byte(msg.Payload), &data)
			if err != nil {
				continue
			}
			err = handler(data)
			if err != nil {
				continue
			}
		}
	}()

	return nil
}

func (p *PubSub[T]) UnsubscribeAll(ctx context.Context, topic string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if sub, ok := p.subscriptions[topic]; ok {
		delete(p.subscriptions, topic)
		return sub.Close()
	}

	return nil
}

func (p *PubSub[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var err error
	for topic, sub := range p.subscriptions {
		err = sub.Close()
		delete(p.subscriptions, topic)
	}

	return err
}
