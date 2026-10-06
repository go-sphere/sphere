// Package cache is the typed key/value contract used by Sphere services, plus
// adapters that sit on top of any driver.
//
// [Cache] combines [Core], [Bulk], [TTL], [Evictor], and [io.Closer]. Drivers
// live in subpackages:
//
//   - cache/memory: in-process, ristretto-backed, highest throughput.
//   - cache/mcache: in-process map with lazy TTL and key listing.
//   - cache/redis: Redis-backed [ByteCache].
//   - cache/badgerdb: persistent, BadgerDB-backed [ByteCache].
//   - cache/nocache: always-miss driver for turning caching off.
//   - cache/nscache: key-prefix wrapper so several logical caches share one
//     backend.
//
// On top of any driver, [NewJsonCache] and [NewCodecCache] turn a [ByteCache]
// into a typed Cache[T], and the loader helpers ([GetEx], [GetJsonEx],
// [Set], [SetJson], …) add read-through fill, JSON encoding, optional
// singleflight, and per-value TTL.
//
// # Usage
//
//	import (
//		"context"
//		"time"
//
//		"github.com/go-sphere/sphere/cache"
//		"github.com/go-sphere/sphere/cache/mcache"
//	)
//
//	type User struct{ Name string }
//
//	backend := mcache.NewByteCache() // any cache.ByteCache driver
//	defer backend.Close()            // the caller owns the backend
//
//	users := cache.NewJsonCache[User](backend)
//	if err := users.SetWithTTL(ctx, "user:1", User{Name: "Ada"}, time.Minute); err != nil {
//		return err
//	}
//	u, found, err := users.Get(ctx, "user:1")
//
//	// Read-through: call the builder on a miss and store its result.
//	u, found, err = cache.GetEx(ctx, users, "user:2", func() (User, error) {
//		return loadUser(ctx, 2)
//	}, cache.WithExpiration(time.Minute))
//
// # Capabilities
//
//   - Single-key CRUD: Set, Get, GetDel, Del, Exists. Get reports a miss as
//     found=false with a nil error.
//   - Batches: MultiSet, MultiGet, MultiDel. Batches are not atomic.
//   - TTL: SetWithTTL / MultiSetWithTTL. expiration > 0 expires; 0 never
//     expires and clears any existing TTL; < 0 returns [ErrInvalidTTL] and
//     writes nothing.
//   - Optional [KeyLister]: implemented by mcache, badgerdb, redis, nocache,
//     [CodecCache], and nscache.NSCache. The ristretto-backed memory driver
//     does not implement it. NSCache.DelAll needs a KeyLister; without one it
//     returns [ErrNotSupported].
//
// # Ownership
//
// A constructor that opens a resource closes it; a constructor that receives
// one does not. Wrappers ([CodecCache], nscache.NSCache) never close the
// injected backend, so one [ByteCache] can back several of them; the caller
// closes the backend.
//
// # DelAll blast radius
//
// The redis driver's DelAll is FlushDB of the selected Redis database, not this
// wrapper's keys. memory and mcache clear the whole process cache. NSCache
// deletes only its namespace. Do not share a Redis DB with mq keys if you call
// DelAll on the cache.
//
// [ErrClosed] is returned after Close only by the memory driver. mcache and
// nocache Close are no-ops and later operations still succeed. redis and
// badgerdb surface their own closed-connection error when the owned client
// or DB was actually closed.
package cache
