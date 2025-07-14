package nocache

import (
	"context"
	"time"
)

// NoCache is a no-operation cache implementation that does not store any data.
type NoCache[T any] struct{}

func NewNoCache[T any]() *NoCache[T] {
	return &NoCache[T]{}
}

func (n *NoCache[T]) Set(ctx context.Context, key string, val T) error {
	return nil
}

func (n *NoCache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	return nil
}

func (n *NoCache[T]) MultiSet(ctx context.Context, valMap map[string]T) error {
	return nil
}

func (n *NoCache[T]) MultiSetWithTTL(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	return nil
}

func (n *NoCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	return zero, false, nil
}

func (n *NoCache[T]) MultiGet(ctx context.Context, keys []string) (map[string]T, error) {
	return make(map[string]T), nil
}

func (n *NoCache[T]) Del(ctx context.Context, key string) error {
	return nil
}

func (n *NoCache[T]) MultiDel(ctx context.Context, keys []string) error {
	return nil
}

func (n *NoCache[T]) DelAll(ctx context.Context) error {
	return nil
}

func (n *NoCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (n *NoCache[T]) Close() error {
	return nil
}

type ByteNoCache = NoCache[[]byte]

func NewByteNoCache() *ByteNoCache {
	return &ByteNoCache{}
}
