// Package nocache is an always-miss cache.Cache that stores nothing.
//
// Sets succeed. Gets return (zero, false, nil). Negative TTL is still
// rejected as cache.ErrInvalidTTL so swapping this in does not hide caller
// bugs. Implements KeyLister with an empty keyspace so NSCache.DelAll keeps
// working when caching is turned off.
package nocache

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/cache"
)

// NoCache is a no-operation cache implementation that does not store any data.
// It implements the Cache interface but all operations are no-ops, useful for disabling caching.
type NoCache[T any] struct{}

// NewNoCache creates a new no-operation cache that doesn't actually cache anything.
func NewNoCache[T any]() *NoCache[T] {
	return &NoCache[T]{}
}

func (n *NoCache[T]) Set(ctx context.Context, key string, val T) error {
	return nil
}

// SetWithTTL still rejects a negative expiration even though nothing is
// stored. NoCache is a drop-in switch for turning caching off, so swapping it
// in must change what is persisted, not which arguments are legal — otherwise
// it silently accepts a TTL every other driver rejects and hides the caller's
// bug until the switch is flipped back.
func (n *NoCache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	return nil
}

func (n *NoCache[T]) MultiSet(ctx context.Context, valMap map[string]T) error {
	return nil
}

func (n *NoCache[T]) MultiSetWithTTL(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	return nil
}

func (n *NoCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	return zero, false, nil
}

func (n *NoCache[T]) GetDel(ctx context.Context, key string) (T, bool, error) {
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

// Keys implements cache.KeyLister and always reports an empty keyspace.
//
// The capability exists so NoCache stays a drop-in switch. nscache.NSCache
// requires a KeyLister to scope DelAll to its own namespace and returns
// ErrNotSupported without one, so turning caching off used to make DelAll start
// failing — on a cache holding nothing at all. The reason NSCache is cautious
// (never wipe keys belonging to a sibling namespace) cannot apply here: there
// are no keys to wipe.
func (n *NoCache[T]) Keys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

func (n *NoCache[T]) Close() error {
	return nil
}

// ByteNoCache is a no-operation cache for byte slices.
type ByteNoCache = NoCache[[]byte]

// NewByteNoCache creates a new no-operation byte cache.
func NewByteNoCache() *ByteNoCache {
	return &ByteNoCache{}
}
