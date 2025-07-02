package memory

import (
	"context"
	"fmt"
	"sync"
)

type Subscription[T any] struct {
	id      string
	handler func(data T) error
	ch      chan T
	done    chan struct{}
}

type PubSub[T any] struct {
	// topic -> subscriptionID -> subscription
	topics map[string]map[string]*Subscription[T]
	mu     sync.RWMutex
	closed bool
	nextID int64
}

func NewPubSub[T any]() *PubSub[T] {
	return &PubSub[T]{
		topics: make(map[string]map[string]*Subscription[T]),
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

func (p *PubSub[T]) Subscribe(ctx context.Context, topic string, handler func(data T) error) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return "", fmt.Errorf("pubsub is closed")
	}
	p.nextID++
	subscriptionID := fmt.Sprintf("sub_%d", p.nextID)

	sub := &Subscription[T]{
		id:      subscriptionID,
		handler: handler,
		ch:      make(chan T, 100),
		done:    make(chan struct{}),
	}

	if p.topics[topic] == nil {
		p.topics[topic] = make(map[string]*Subscription[T])
	}
	p.topics[topic][subscriptionID] = sub

	go p.handleSubscription(sub)

	return subscriptionID, nil
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

func (p *PubSub[T]) Unsubscribe(ctx context.Context, topic string, subscriptionID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("pubsub is closed")
	}

	if subscribers, exists := p.topics[topic]; exists {
		if sub, sExists := subscribers[subscriptionID]; sExists {

			close(sub.done)
			delete(subscribers, subscriptionID)

			if len(subscribers) == 0 {
				delete(p.topics, topic)
			}
			return nil
		}
	}

	return fmt.Errorf("subscription %s not found in topic %s", subscriptionID, topic)
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
