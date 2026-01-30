package nscache

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/cache"
)

// NSCache is a namespaced cache wrapper.
type NSCache[S any] struct {
	namespace string
	cache     cache.Cache[S]
}

// NewNSCache creates a new namespaced cache.
func NewNSCache[S any](namespace string, cache cache.Cache[S]) *NSCache[S] {
	return &NSCache[S]{
		namespace: namespace,
		cache:     cache,
	}
}

func (n *NSCache[S]) keygen(key string) string {
	return n.namespace + ":" + key
}

func (n *NSCache[S]) Set(ctx context.Context, key string, val S) error {
	return n.cache.Set(ctx, n.keygen(key), val)
}

func (n *NSCache[S]) Get(ctx context.Context, key string) (S, bool, error) {
	return n.cache.Get(ctx, n.keygen(key))
}

func (n *NSCache[S]) GetDel(ctx context.Context, key string) (S, bool, error) {
	return n.cache.GetDel(ctx, n.keygen(key))
}

func (n *NSCache[S]) Del(ctx context.Context, key string) error {
	return n.cache.Del(ctx, n.keygen(key))
}

func (n *NSCache[S]) Exists(ctx context.Context, key string) (bool, error) {
	return n.cache.Exists(ctx, n.keygen(key))
}

func (n *NSCache[S]) MultiSet(ctx context.Context, valMap map[string]S) error {
	return n.cache.MultiSet(ctx, func() map[string]S {
		mapped := make(map[string]S, len(valMap))
		for k, v := range valMap {
			mapped[n.keygen(k)] = v
		}
		return mapped
	}())
}

func (n *NSCache[S]) MultiGet(ctx context.Context, keys []string) (map[string]S, error) {
	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = n.keygen(k)
	}
	res, err := n.cache.MultiGet(ctx, prefixedKeys)
	if err != nil {
		return nil, err
	}
	unprefixedRes := make(map[string]S, len(res))
	for k, v := range res {
		unprefixedKey := k[len(n.namespace)+1:]
		unprefixedRes[unprefixedKey] = v
	}
	return unprefixedRes, nil
}

func (n *NSCache[S]) MultiDel(ctx context.Context, keys []string) error {
	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = n.keygen(k)
	}
	return n.cache.MultiDel(ctx, prefixedKeys)
}

func (n *NSCache[S]) SetWithTTL(ctx context.Context, key string, val S, expiration time.Duration) error {
	return n.cache.SetWithTTL(ctx, n.keygen(key), val, expiration)
}

func (n *NSCache[S]) MultiSetWithTTL(ctx context.Context, valMap map[string]S, expiration time.Duration) error {
	prefixedValMap := make(map[string]S, len(valMap))
	for k, v := range valMap {
		prefixedValMap[n.keygen(k)] = v
	}
	return n.cache.MultiSetWithTTL(ctx, prefixedValMap, expiration)
}

func (n *NSCache[S]) DelAll(ctx context.Context) error {
	return n.cache.DelAll(ctx)
}

func (n *NSCache[S]) Close() error {
	return n.cache.Close()
}
