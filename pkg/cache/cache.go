package cache

import (
	"context"
	"io"
	"time"
)

type Cache[S any] interface {
	Set(ctx context.Context, key string, val S, expiration time.Duration) error
	Get(ctx context.Context, key string) (S, error)
	MultiSet(ctx context.Context, valMap map[string]S, expiration time.Duration) error
	MultiGet(ctx context.Context, keys []string) (map[string]S, error)
	Del(ctx context.Context, keys ...string) error
	DelAll(ctx context.Context) error
	io.Closer
}

type ByteCache Cache[[]byte]
