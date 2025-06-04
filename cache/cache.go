package cache

import (
	"context"
	"io"
	"time"
)

type Cache[S any] interface {
	Set(ctx context.Context, key string, val S) error
	SetWithTTL(ctx context.Context, key string, val S, expiration time.Duration) error
	MultiSet(ctx context.Context, valMap map[string]S) error
	MultiSetWithTTL(ctx context.Context, valMap map[string]S, expiration time.Duration) error

	// Get retrieves the value for the given key, returns (nil, nil) if the key does not exist
	Get(ctx context.Context, key string) (*S, error)
	MultiGet(ctx context.Context, keys []string) (map[string]S, error)

	Del(ctx context.Context, key string) error
	MultiDel(ctx context.Context, keys []string) error
	DelAll(ctx context.Context) error

	io.Closer
}

type ByteCache Cache[[]byte]
