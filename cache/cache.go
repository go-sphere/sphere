package cache

import (
	"context"
	"io"
	"time"
)

const (
	NeverExpire time.Duration = -1
)

type Cache[S any] interface {
	// Set sets the value for the given key with an expiration time, if expiration is -1, never expires
	Set(ctx context.Context, key string, val S, expiration time.Duration) error
	// Get retrieves the value for the given key, returns (nil, nil) if the key does not exist
	Get(ctx context.Context, key string) (*S, error)
	Del(ctx context.Context, key string) error
	MultiSet(ctx context.Context, valMap map[string]S, expiration time.Duration) error
	MultiGet(ctx context.Context, keys []string) (map[string]S, error)
	MultiDel(ctx context.Context, keys []string) error
	DelAll(ctx context.Context) error
	io.Closer
}

type ByteCache Cache[[]byte]
