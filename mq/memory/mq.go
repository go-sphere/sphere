package memory

import "errors"

// MessageQueue combines both queue and publish-subscribe functionality in a single memory-based implementation.
// It provides both point-to-point messaging and broadcast messaging capabilities.
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

func (p *MessageQueue[T]) Close() error {
	return errors.Join(
		p.Queue.Close(),
		p.PubSub.Close(),
	)
}
