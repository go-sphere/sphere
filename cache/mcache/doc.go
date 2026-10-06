// Package mcache is a mutex-protected map cache.Cache driver with lazy TTL.
//
// Use it for small process-local caches that need key listing
// (cache.KeyLister) or deterministic behavior in tests. Entry points:
// [NewMapCache] (alias [NewCache]), [NewMapCacheWithCapacity], and
// [NewByteCache] for a cache.ByteCache.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/cache/mcache"
//
//	c := mcache.NewMapCache[int]()
//	defer c.Close() // no-op, kept for the cache.Cache contract
//
//	if err := c.SetWithTTL(ctx, "visits", 1, time.Minute); err != nil {
//		return err
//	}
//	v, found, err := c.Get(ctx, "visits")
//
//	// In long-running processes, reclaim expired entries on a timer:
//	c.Trim()
//
// # Behavior
//
// There is no background janitor: expired entries are dropped on
// Get/GetDel/MultiGet and bulk-reclaimed by [Map.Trim]. There is no capacity
// cap. Close is a no-op; later operations still succeed. []byte values are
// cloned; other types are stored as the caller's value. Get takes a write
// lock so it can delete expired keys; [Map.Count] is a cheap read and does not
// reclaim, so keep Trim on a timer in long-running processes.
package mcache
