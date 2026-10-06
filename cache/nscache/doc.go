// Package nscache prefixes every key with "<namespace>:" so several logical
// caches can share one cache.Cache.
//
// Use [NewNSCacheChecked] for namespaces from configuration or other external
// input, and [NewNSCache] for trusted constant namespaces.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/cache/mcache"
//		"github.com/go-sphere/sphere/cache/nscache"
//	)
//
//	backend := mcache.NewByteCache()
//	defer backend.Close() // the caller owns the shared backend
//
//	sessions, err := nscache.NewNSCacheChecked[[]byte]("session", backend)
//	if err != nil {
//		return err
//	}
//	_ = sessions.Set(ctx, "abc", []byte("v")) // stored as "session:abc"
//	_ = sessions.DelAll(ctx)                  // deletes only "session:*"
//
// DelAll and Keys are namespace-scoped and need cache.KeyLister on the inner
// cache (mcache, badgerdb, redis, nocache, cache.CodecCache if its inner cache
// is a lister); otherwise they return cache.ErrNotSupported. The ristretto
// memory driver is not a lister. Close is a no-op. Namespaces must not contain
// ":": "a" and "a:b" overlap under prefix "a:".
package nscache
