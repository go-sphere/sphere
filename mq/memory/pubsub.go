package memory

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/mq"
)

type subscription[T any] struct {
	parent  *PubSub[T]
	topic   string
	handler mq.Handler[T]
	ch      chan T
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	stop    sync.Once
}

func (s *subscription[T]) Stop() error {
	s.parent.remove(s)
	s.requestStop()
	return nil
}

func (s *subscription[T]) Done() <-chan struct{} {
	return s.done
}

func (s *subscription[T]) requestStop() {
	s.stop.Do(s.cancel)
}

// PubSub implements process-local best-effort publish-subscribe delivery.
// Each subscription has a bounded buffer; Broadcast drops rather than blocks
// when one subscriber falls behind.
type PubSub[T any] struct {
	identifier string
	queueSize  int
	topics     map[string][]*subscription[T]

	mu       sync.RWMutex
	closed   bool
	done     chan struct{}
	stopOnce sync.Once
	handlers sync.WaitGroup
}

// NewPubSub creates an in-memory PubSub. The default per-subscription buffer
// holds 100 messages.
func NewPubSub[T any](opt ...Option) *PubSub[T] {
	opts := newOptions(opt...)
	return &PubSub[T]{
		identifier: opts.identifier,
		queueSize:  opts.queueSize,
		topics:     make(map[string][]*subscription[T]),
		done:       make(chan struct{}),
	}
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

// Broadcast delivers data to every current subscriber without blocking. A
// full subscriber buffer drops the message and logs a warning.
func (p *PubSub[T]) Broadcast(ctx context.Context, topic string, data T) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return mq.ErrPubSubClosed
	}
	subscribers := slices.Clone(p.topics[topic])
	p.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	for _, sub := range subscribers {
		select {
		case <-sub.ctx.Done():
		case sub.ch <- data:
		default:
			log.Warn("pubsub broadcast dropped message: subscriber queue full", log.String("topic", topic))
		}
	}
	return nil
}

// Subscribe registers handler on topic. ctx controls the subscription's full
// lifetime and is passed to every handler invocation.
func (p *PubSub[T]) Subscribe(ctx context.Context, topic string, handler mq.Handler[T]) (mq.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("mq memory: subscription context is required")
	}
	if handler == nil {
		return nil, errors.New("mq memory: subscription handler is required")
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

	subCtx, cancel := context.WithCancel(ctx)
	sub := &subscription[T]{
		parent:  p,
		topic:   topic,
		handler: handler,
		ch:      make(chan T, p.queueSize),
		ctx:     subCtx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		cancel()
		return nil, mq.ErrPubSubClosed
	}
	p.topics[topic] = append(p.topics[topic], sub)
	p.handlers.Add(1)
	p.mu.Unlock()

	go p.run(sub)
	return sub, nil
}

func (p *PubSub[T]) run(sub *subscription[T]) {
	defer p.handlers.Done()
	defer close(sub.done)
	defer sub.requestStop()
	defer p.remove(sub)

	for {
		select {
		case <-sub.ctx.Done():
			return
		case data := <-sub.ch:
			// Cancellation wins over buffered work. A handler that already started
			// may finish, but shutdown does not drain queued PubSub messages.
			if sub.ctx.Err() != nil {
				return
			}
			p.dispatch(sub.ctx, sub.handler, data)
		}
	}
}

func (p *PubSub[T]) dispatch(ctx context.Context, handler mq.Handler[T], data T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error("recovered from panic in subscription handler", log.Any("error", recovered))
		}
	}()
	if err := handler(ctx, data); err != nil {
		log.Error("subscription handler error", log.Err(err))
	}
}

func (p *PubSub[T]) remove(target *subscription[T]) {
	p.mu.Lock()
	subs := p.topics[target.topic]
	for i, sub := range subs {
		if sub != target {
			continue
		}
		subs = slices.Delete(subs, i, i+1)
		if len(subs) == 0 {
			delete(p.topics, target.topic)
		} else {
			p.topics[target.topic] = subs
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
	subs := p.topics[topic]
	delete(p.topics, topic)
	p.mu.Unlock()

	for _, sub := range subs {
		sub.requestStop()
	}
	return subscriptionsDone(subs), nil
}

// RequestStop prevents new operations and requests cancellation of every
// current subscription. It does not wait for handlers and is safe to call from
// one. External lifecycle owners should call Stop.
func (p *PubSub[T]) RequestStop() error {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		var subs []*subscription[T]
		for _, topicSubs := range p.topics {
			subs = append(subs, topicSubs...)
		}
		clear(p.topics)
		p.mu.Unlock()

		for _, sub := range subs {
			sub.requestStop()
		}
		go func() {
			p.handlers.Wait()
			close(p.done)
		}()
	})
	return nil
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
		return fmt.Errorf("mq memory: stop: %w", errors.Join(stopErr, ctx.Err()))
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
