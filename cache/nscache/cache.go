package nscache

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/go-sphere/sphere/cache"
)

// ErrInvalidNamespace is returned by NewNSCacheChecked when namespace is
// empty or contains ":". Colons are reserved as the separator between namespace
// and key.
var ErrInvalidNamespace = errors.New("nscache: namespace must not be empty or contain ':'")

// ErrNilCache is returned by NewNSCacheChecked when cache is nil (including a
// typed nil).
var ErrNilCache = errors.New("nscache: cache is required")

// NSCache is a namespaced cache wrapper that prefixes every key with
// "<namespace>:" before delegating to an underlying cache.Cache.
//
// DelAll deletes only keys belonging to this namespace, and Close leaves the
// backend open, so multiple NSCache instances can safely share a single
// backend. DelAll requires the wrapped cache to implement cache.KeyLister
// (mcache, badgerdb, redis, nocache, CodecCache if the inner is a lister);
// otherwise it returns cache.ErrNotSupported.
//
// Wrapping the shared cache in a namespace is the safe way to expose a
// "clear cache" management action: the bare redis driver's DelAll is a
// FlushDB of the selected database, which would also wipe mq/scheduler keys
// sharing that DB.
//
// Namespaces must be flat and must not contain ":". Isolation is by key
// prefix, so with namespaces "a" and "a:b" the keys of "a:b" also carry the
// "a:" prefix, and DelAll or Keys on "a" would reach into "a:b".
type NSCache[S any] struct {
	namespace string
	prefix    string
	cache     cache.Cache[S]
}

// NewNSCache wraps cache with a key prefix of namespace + ":". It assumes the
// namespace is trusted and does not contain ":". Use NewNSCacheChecked for a
// namespace read from configuration or other external input.
func NewNSCache[S any](namespace string, cache cache.Cache[S]) *NSCache[S] {
	return &NSCache[S]{
		namespace: namespace,
		prefix:    namespace + ":",
		cache:     cache,
	}
}

// NewNSCacheChecked wraps cache with a validated namespace and non-nil backend.
// An empty namespace or one containing ":" returns ErrInvalidNamespace because
// it would overlap another namespace under the prefix-based Keys and DelAll
// implementation. A nil (or typed-nil) cache returns ErrNilCache.
func NewNSCacheChecked[S any](namespace string, cache cache.Cache[S]) (*NSCache[S], error) {
	if isNilCache(cache) {
		return nil, ErrNilCache
	}
	if namespace == "" || strings.Contains(namespace, ":") {
		return nil, ErrInvalidNamespace
	}
	return NewNSCache(namespace, cache), nil
}

func isNilCache[S any](c cache.Cache[S]) bool {
	if c == nil {
		return true
	}
	v := reflect.ValueOf(c)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}

func (n *NSCache[S]) keygen(key string) string {
	return n.prefix + key
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
	if len(valMap) == 0 {
		return n.cache.MultiSet(ctx, nil)
	}
	prefixedValMap := make(map[string]S, len(valMap))
	for k, v := range valMap {
		prefixedValMap[n.keygen(k)] = v
	}
	return n.cache.MultiSet(ctx, prefixedValMap)
}

func (n *NSCache[S]) MultiGet(ctx context.Context, keys []string) (map[string]S, error) {
	if len(keys) == 0 {
		res, err := n.cache.MultiGet(ctx, nil)
		if err != nil {
			return nil, err
		}
		if len(res) == 0 {
			return make(map[string]S), nil
		}
	}
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
		unprefixedKey := strings.TrimPrefix(k, n.prefix)
		unprefixedRes[unprefixedKey] = v
	}
	return unprefixedRes, nil
}

func (n *NSCache[S]) MultiDel(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return n.cache.MultiDel(ctx, nil)
	}
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
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	if len(valMap) == 0 {
		return n.cache.MultiSetWithTTL(ctx, nil, expiration)
	}
	prefixedValMap := make(map[string]S, len(valMap))
	for k, v := range valMap {
		prefixedValMap[n.keygen(k)] = v
	}
	return n.cache.MultiSetWithTTL(ctx, prefixedValMap, expiration)
}

// DelAll removes every key in this namespace by listing them with Keys and
// deleting them with MultiDel in batches of at most 1000 keys, both of which
// stay inside the namespace. Batching keeps each delete within backend limits
// (badger's transaction size, a single redis DEL's argument count). It
// therefore inherits the Keys requirement: the wrapped cache must implement
// cache.KeyLister, otherwise DelAll returns cache.ErrNotSupported rather than
// risk wiping keys belonging to a sibling namespace on the same backend.
//
// The listing and the deletion are not atomic: keys written in between
// survive. A failing batch stops DelAll; earlier batches stay deleted.
func (n *NSCache[S]) DelAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	lister, ok := n.cache.(cache.KeyLister)
	if !ok {
		return cache.ErrNotSupported
	}
	// lister.Keys already returns fully prefixed keys, so pass them straight
	// to MultiDel without stripping and re-prefixing every key.
	keys, err := lister.Keys(ctx, n.prefix)
	if err != nil {
		return err
	}
	for len(keys) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := keys[:min(len(keys), delAllBatchSize)]
		if err := n.cache.MultiDel(ctx, batch); err != nil {
			return err
		}
		keys = keys[len(batch):]
	}
	return nil
}

// delAllBatchSize bounds how many keys DelAll passes to one MultiDel call.
const delAllBatchSize = 1000

// Keys lists the keys in this namespace whose unprefixed name starts with
// prefix. The namespace prefix is stripped from the result, so the returned
// keys can be passed straight back to this cache's own methods and an NSCache
// can itself be wrapped by another NSCache. The wrapped cache must implement
// cache.KeyLister; otherwise Keys returns cache.ErrNotSupported.
func (n *NSCache[S]) Keys(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
		trimmed = append(trimmed, strings.TrimPrefix(k, n.prefix))
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
