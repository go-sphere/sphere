package memory

import "errors"

type MessageQueue[T any] struct {
	*Queue[T]
	*PubSub[T]
}

func NewMessageQueue[T any]() *MessageQueue[T] {
	return &MessageQueue[T]{
		Queue:  NewQueue[T](),
		PubSub: NewPubSub[T](),
	}
}

func (p *MessageQueue[T]) Close() error {
	return errors.Join(
		p.Queue.Close(),
		p.PubSub.Close(),
	)
}
