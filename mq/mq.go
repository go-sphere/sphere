package mq

import (
	"context"
	"io"
)

type Queue[T any] interface {
	Publish(ctx context.Context, topic string, data T) error
	Consume(ctx context.Context, topic string) (T, error)
	PurgeQueue(ctx context.Context, topic string) error
	io.Closer
}

type PubSub[T any] interface {
	Broadcast(ctx context.Context, topic string, data T) error
	Subscribe(ctx context.Context, topic string, handler func(data T) error) error
	UnsubscribeAll(ctx context.Context, topic string) error
	io.Closer
}

type MessageQueue[T any] interface {
	Queue[T]
	PubSub[T]
}
