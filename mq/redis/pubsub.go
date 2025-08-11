package redis

import (
	"context"
	"sync"

	"github.com/TBXark/sphere/core/codec"
	"github.com/TBXark/sphere/log"
	"github.com/redis/go-redis/v9"
)

type PubSub[T any] struct {
	client *redis.Client
	codec  codec.Codec

	subscriptions map[string]*redis.PubSub
	mu            sync.Mutex
}

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
	return p.client.Publish(ctx, topic, data).Err()
}

func (p *PubSub[T]) Subscribe(ctx context.Context, topic string, handler func(data T) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	sub := p.client.Subscribe(ctx, topic)
	p.subscriptions[topic] = sub

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("recovered from panic in subscription handler", log.Any("recover", r))
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
