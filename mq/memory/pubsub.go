package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-sphere/sphere/log"
)

// Subscription represents an active subscription to a topic with its associated handler and channels.
type Subscription[T any] struct {
	handler func(data T) error
	ch      chan T
	done    chan struct{}
}

// PubSub implements an in-memory publish-subscribe message system with typed message support.
// It broadcasts messages to all active subscribers of a topic.
type PubSub[T any] struct {
	queueSize int
	topics    map[string][]*Subscription[T]

	mu     sync.RWMutex
	closed bool
}

// NewPubSub creates a new memory-based publish-subscribe system with the specified options.
// The default queue size is 100 messages per subscription.
func NewPubSub[T any](opt ...Option) *PubSub[T] {
	opts := newOptions(opt...)
	return &PubSub[T]{
		queueSize: opts.queueSize,
		topics:    make(map[string][]*Subscription[T]),
	}
}

func (p *PubSub[T]) Broadcast(ctx context.Context, topic string, data T) error {
	p.mu.RLock()
	subscribers, exists := p.topics[topic]
	closed := p.closed
	p.mu.RUnlock()

	if closed {
		return fmt.Errorf("pubsub is closed")
	}

	if !exists || len(subscribers) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	for _, sub := range subscribers {
		wg.Add(1)
		go func(s *Subscription[T]) {
			defer wg.Done()
			select {
			case s.ch <- data:
			case <-ctx.Done():
			case <-s.done:
			}
		}(sub)
	}

	wg.Wait()
	return nil
}

func (p *PubSub[T]) Subscribe(ctx context.Context, topic string, handler func(data T) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("pubsub is closed")
	}

	sub := &Subscription[T]{
		handler: handler,
		ch:      make(chan T, p.queueSize),
		done:    make(chan struct{}),
	}
	p.topics[topic] = append(p.topics[topic], sub)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("recovered from panic in subscription handler", log.Any("error", r))
			}
		}()
		p.handleSubscription(sub)
	}()

	return nil
}

func (p *PubSub[T]) handleSubscription(sub *Subscription[T]) {
	for {
		select {
		case data := <-sub.ch:
			if err := sub.handler(data); err != nil {
				fmt.Printf("handler error: %v\n", err)
			}
		case <-sub.done:
			return
		}
	}
}

func (p *PubSub[T]) UnsubscribeAll(ctx context.Context, topic string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("pubsub is closed")
	}

	if subscribers, exists := p.topics[topic]; exists {
		for _, sub := range subscribers {
			close(sub.done)
		}
		delete(p.topics, topic)
	}

	return nil
}

func (p *PubSub[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true

	for _, subscribers := range p.topics {
		for _, sub := range subscribers {
			close(sub.done)
		}
	}
	return nil
}
