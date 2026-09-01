package redis

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/mq"
	"github.com/redis/go-redis/v9"
)

// ErrPubSubClosed is kept as an alias for callers that previously matched the
// driver-specific error. New code should match mq.ErrPubSubClosed.
var ErrPubSubClosed = mq.ErrPubSubClosed

type subscription[T any] struct {
	parent  *PubSub[T]
	topic   string
	handler mq.Handler[T]
	raw     *redis.PubSub
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	stop    sync.Once
	stopErr error
}

func (s *subscription[T]) Stop() error {
	s.parent.remove(s)
	return s.requestStop()
}

func (s *subscription[T]) Done() <-chan struct{} {
	return s.done
}

func (s *subscription[T]) requestStop() error {
	s.stop.Do(func() {
		s.cancel()
		s.stopErr = s.raw.Close()
	})
	return s.stopErr
}

// PubSub implements typed best-effort delivery over Redis Pub/Sub. The Redis
// client is injected and remains owned by the caller.
type PubSub[T any] struct {
	identifier string
	client     *redis.Client
	codec      codec.Codec

	mu            sync.RWMutex
	subscriptions map[string][]*subscription[T]
	closed        bool
	done          chan struct{}
	stopOnce      sync.Once
	stopErr       error
	handlers      sync.WaitGroup
}

// NewPubSub creates a Redis PubSub. WithClient is required; the PubSub never
// closes that shared Redis client.
func NewPubSub[T any](opt ...Option) (*PubSub[T], error) {
	opts := newOptions(opt...)
	if err := opts.validate(); err != nil {
		return nil, err
	}
	return &PubSub[T]{
		identifier:    opts.identifier,
		client:        opts.client,
		codec:         opts.codec,
		subscriptions: make(map[string][]*subscription[T]),
		done:          make(chan struct{}),
	}, nil
}

// Identifier returns the task.Task identifier configured with WithIdentifier.
func (p *PubSub[T]) Identifier() string {
	return p.identifier
}

// Start blocks until Stop completes or ctx is cancelled. Subscribe and
// Broadcast are available before Start; Start is only the task lifecycle wait.
// A caller running Start outside task.Group must still call Stop for cleanup.
func (p *PubSub[T]) Start(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	default:
	}
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Broadcast publishes to topic. Calls that begin after RequestStop return
// mq.ErrPubSubClosed even though the injected Redis client remains usable.
func (p *PubSub[T]) Broadcast(ctx context.Context, topic string, data T) error {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()
	if closed {
		return mq.ErrPubSubClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := p.codec.Marshal(data)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, topic, raw).Err()
}

// Subscribe establishes a Redis subscription and starts handler delivery. ctx
// controls both setup and the returned subscription's full lifetime.
func (p *PubSub[T]) Subscribe(ctx context.Context, topic string, handler mq.Handler[T]) (mq.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("redis mq: subscription context is required")
	}
	if handler == nil {
		return nil, errors.New("redis mq: subscription handler is required")
	}
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()
	if closed {
		return nil, mq.ErrPubSubClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	raw := p.client.Subscribe(ctx, topic)
	if _, err := raw.Receive(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = raw.Close()
		return nil, err
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := &subscription[T]{
		parent:  p,
		topic:   topic,
		handler: handler,
		raw:     raw,
		ctx:     subCtx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		_ = sub.requestStop()
		return nil, mq.ErrPubSubClosed
	}
	p.subscriptions[topic] = append(p.subscriptions[topic], sub)
	p.handlers.Add(1)
	p.mu.Unlock()

	go p.run(sub)
	return sub, nil
}

func (p *PubSub[T]) run(sub *subscription[T]) {
	defer p.handlers.Done()
	defer close(sub.done)
	defer func() { _ = sub.requestStop() }()
	defer p.remove(sub)

	messages := sub.raw.Channel()
	for {
		select {
		case <-sub.ctx.Done():
			return
		case msg, ok := <-messages:
			if !ok {
				return
			}
			if sub.ctx.Err() != nil {
				return
			}
			var data T
			if err := p.codec.Unmarshal([]byte(msg.Payload), &data); err != nil {
				log.Error("failed to unmarshal subscription message", log.Err(err), log.String("topic", sub.topic))
				continue
			}
			p.dispatch(sub.ctx, sub.topic, sub.handler, data)
		}
	}
}

func (p *PubSub[T]) dispatch(ctx context.Context, topic string, handler mq.Handler[T], data T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error("recovered from panic in subscription handler", log.Any("error", recovered), log.String("topic", topic))
		}
	}()
	if err := handler(ctx, data); err != nil {
		log.Error("subscription handler error", log.Err(err), log.String("topic", topic))
	}
}

func (p *PubSub[T]) remove(target *subscription[T]) {
	p.mu.Lock()
	subs := p.subscriptions[target.topic]
	for i, sub := range subs {
		if sub != target {
			continue
		}
		subs = slices.Delete(subs, i, i+1)
		if len(subs) == 0 {
			delete(p.subscriptions, target.topic)
		} else {
			p.subscriptions[target.topic] = subs
		}
		break
	}
	p.mu.Unlock()
}

// StopTopic requests cancellation of the topic's current subscriptions and
// returns a channel that closes after their handlers return.
func (p *PubSub[T]) StopTopic(topic string) (<-chan struct{}, error) {
	p.mu.Lock()
	if p.closed {
		done := p.done
		p.mu.Unlock()
		return done, nil
	}
	subs := p.subscriptions[topic]
	delete(p.subscriptions, topic)
	p.mu.Unlock()

	var stopErr error
	for _, sub := range subs {
		stopErr = errors.Join(stopErr, sub.requestStop())
	}
	return subscriptionsDone(subs), stopErr
}

// RequestStop prevents new operations and requests cancellation of every
// current subscription. It does not wait for handlers and is safe to call from
// one. External lifecycle owners should call Stop.
func (p *PubSub[T]) RequestStop() error {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		var subs []*subscription[T]
		for _, topicSubs := range p.subscriptions {
			subs = append(subs, topicSubs...)
		}
		clear(p.subscriptions)
		p.mu.Unlock()

		for _, sub := range subs {
			p.stopErr = errors.Join(p.stopErr, sub.requestStop())
		}
		go func() {
			p.handlers.Wait()
			close(p.done)
		}()
	})
	return p.stopErr
}

// Done closes after RequestStop and after every subscription handler has
// returned.
func (p *PubSub[T]) Done() <-chan struct{} {
	return p.done
}

// Stop implements task.Task. It requests shutdown and waits for quiescence or
// ctx cancellation. A handler must call RequestStop instead of waiting on
// itself through Stop.
func (p *PubSub[T]) Stop(ctx context.Context) error {
	stopErr := p.RequestStop()
	select {
	case <-p.done:
		return stopErr
	default:
	}
	select {
	case <-p.done:
		return stopErr
	case <-ctx.Done():
		return fmt.Errorf("redis mq: stop: %w", errors.Join(stopErr, ctx.Err()))
	}
}

func subscriptionsDone[T any](subs []*subscription[T]) <-chan struct{} {
	done := make(chan struct{})
	if len(subs) == 0 {
		close(done)
		return done
	}
	go func() {
		for _, sub := range subs {
			<-sub.done
		}
		close(done)
	}()
	return done
}
