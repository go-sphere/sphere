package redis

import "errors"

type MessageQueue[T any] struct {
	*Queue[T]
	*PubSub[T]
}

func NewMessageQueue[T any](opt ...Option) (*MessageQueue[T], error) {
	queue, err := NewQueue[T](opt...)
	if err != nil {
		return nil, err
	}
	pubSub, err := NewPubSub[T](opt...)
	if err != nil {
		return nil, err
	}
	return &MessageQueue[T]{
		Queue:  queue,
		PubSub: pubSub,
	}, nil
}

func (p *MessageQueue[T]) Close() error {
	return errors.Join(
		p.Queue.Close(),
		p.PubSub.Close(),
	)
}
