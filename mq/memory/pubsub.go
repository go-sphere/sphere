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
	// Checked once up front rather than inside the send select below: with a
	// default branch the select never blocks, so a ctx case there would only
	// compete at random with a send that could have succeeded.
	if err := ctx.Err(); err != nil {
		return err
	}
	// Copy the slice: p.topics may be mutated by Subscribe/UnsubscribeAll once the
	// read lock is released.
	subscribers = append([]*Subscription[T](nil), subscribers...)

	// Delivery is a non-blocking send, so this loop runs inline. Fanning it out
	// across a goroutine per subscriber would add scheduling cost per message
	// without ever overlapping a wait.
	for _, sub := range subscribers {
		// The default branch provides back pressure protection: when a
		// subscriber's buffered channel is full (slow or stalled consumer) the
		// message is dropped and logged instead of blocking the whole Broadcast,
		// which would otherwise hang forever under a background context.
		select {
		case sub.ch <- data:
		default:
			log.Warn("pubsub broadcast dropped message: subscriber queue full", log.String("topic", topic))
		}
	}
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

	go p.handleSubscription(sub)

	return nil
}

func (p *PubSub[T]) handleSubscription(sub *Subscription[T]) {
	for {
		select {
		case data := <-sub.ch:
			p.dispatch(sub, data)
		case <-sub.done:
			return
		}
	}
}

// dispatch invokes the subscriber handler for a single message with panic
// recovery scoped to that message. A panic in one message is logged and
// consumption continues, so a misbehaving handler can no longer kill the
// consumer goroutine and turn the subscription into a zombie.
func (p *PubSub[T]) dispatch(sub *Subscription[T], data T) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("recovered from panic in subscription handler", log.Any("error", r))
		}
	}()
	if err := sub.handler(data); err != nil {
		log.Error("subscription handler error", log.Err(err))
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
