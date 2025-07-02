package memory

import "errors"

type MessageQueue[T any] struct {
	*Queue[T]
	*PubSub[T]
}

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
