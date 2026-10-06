package memory

import (
	"context"
	"errors"
)

// MessageQueue embeds the memory Queue and PubSub. Close joins both halves.
type MessageQueue[T any] struct {
	*Queue[T]
	*PubSub[T]
}

// NewMessageQueue creates a new memory-based message queue that supports both queue and pub/sub operations.
// The same options apply to both the underlying queue and pub/sub components.
func NewMessageQueue[T any](opt ...Option) *MessageQueue[T] {
	return &MessageQueue[T]{
		Queue:  NewQueue[T](opt...),
		PubSub: NewPubSub[T](opt...),
	}
}

// Close closes the Queue and requests PubSub stop. It does not wait for PubSub
// handlers; task lifecycle owners should call Stop.
func (p *MessageQueue[T]) Close() error {
	return errors.Join(
		p.Queue.Close(),
		p.RequestStop(),
	)
}

// Stop implements task.Task by closing the Queue and waiting for PubSub
// handlers to return or ctx to expire.
func (p *MessageQueue[T]) Stop(ctx context.Context) error {
	return errors.Join(
		p.Queue.Close(),
		p.PubSub.Stop(ctx),
	)
}
