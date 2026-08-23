// Package nscache prefixes every key with "<namespace>:" so several logical
// caches can share one cache.Cache.
//
// DelAll/Keys are namespace-scoped and need cache.KeyLister on the inner
// cache (mcache, badgerdb, redis, nocache, CodecCache if the inner is a
// lister). The ristretto memory driver is not a lister. Close is a no-op.
// Namespaces must not contain ":": "a" and "a:b" overlap under prefix "a:".
package nscache

import (
	"context"
	"strings"
	"time"

	"github.com/go-sphere/sphere/cache"
)

// NSCache is a namespaced cache wrapper that prefixes every key with
// "<namespace>:" before delegating to an underlying cache.Cache.
//
// DelAll deletes only keys belonging to this namespace, and Close leaves the
// backend open, so multiple NSCache instances can safely share a single
// backend. DelAll requires the wrapped cache to implement cache.KeyLister
// (mcache, badgerdb, redis, nocache, CodecCache if the inner is a lister);
// otherwise it returns cache.ErrNotSupported.
//
// Namespaces must be flat and must not contain ":". Isolation is by key
// prefix, so with namespaces "a" and "a:b" the keys of "a:b" also carry the
// "a:" prefix, and DelAll or Keys on "a" would reach into "a:b".
type NSCache[S any] struct {
	namespace string
	cache     cache.Cache[S]
}

// NewNSCache wraps cache with a key prefix of namespace + ":". It does not
// reject a namespace that contains ":".
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
	prefix := n.namespace + ":"
	for k, v := range res {
		unprefixedKey := strings.TrimPrefix(k, prefix)
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

// DelAll removes every key in this namespace by listing them with Keys and
// deleting them with MultiDel, both of which stay inside the namespace. It
// therefore inherits the Keys requirement: the wrapped cache must implement
// cache.KeyLister, otherwise DelAll returns cache.ErrNotSupported rather than
// risk wiping keys belonging to a sibling namespace on the same backend.
//
// The listing and the deletion are not atomic: keys written in between
// survive.
func (n *NSCache[S]) DelAll(ctx context.Context) error {
	keys, err := n.Keys(ctx, "")
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return n.MultiDel(ctx, keys)
}

// Keys lists the keys in this namespace whose unprefixed name starts with
// prefix. The namespace prefix is stripped from the result, so the returned
// keys can be passed straight back to this cache's own methods and an NSCache
// can itself be wrapped by another NSCache. The wrapped cache must implement
// cache.KeyLister; otherwise Keys returns cache.ErrNotSupported.
func (n *NSCache[S]) Keys(ctx context.Context, prefix string) ([]string, error) {
	lister, ok := n.cache.(cache.KeyLister)
	if !ok {
		return nil, cache.ErrNotSupported
	}
	keys, err := lister.Keys(ctx, n.keygen(prefix))
	if err != nil {
		return nil, err
	}
	trimmed := make([]string, 0, len(keys))
	for _, k := range keys {
		trimmed = append(trimmed, strings.TrimPrefix(k, n.namespace+":"))
	}
	return trimmed, nil
}

// Close is a no-op. The wrapped cache is injected, not created here, so this
// wrapper never owns it: closing one namespace must not take down the sibling
// namespaces sharing the same backend. The caller keeps ownership and closes
// the backend itself, the same rule the driver constructors follow.
func (n *NSCache[S]) Close() error {
	return nil
}
