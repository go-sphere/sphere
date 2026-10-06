// Package nocache is an always-miss cache.Cache that stores nothing.
//
// Use it to turn caching off without changing callers: swap [NewNoCache] or
// [NewByteNoCache] in where a real driver would go.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/cache/nocache"
//
//	c := nocache.NewNoCache[string]()
//	_ = c.Set(ctx, "k", "v")    // succeeds, stores nothing
//	_, found, _ := c.Get(ctx, "k") // found == false
//
// Sets succeed. Gets return (zero, false, nil). Negative TTL is still
// rejected as cache.ErrInvalidTTL so swapping this in does not hide caller
// bugs. It implements cache.KeyLister with an empty keyspace so
// nscache.NSCache.DelAll keeps working when caching is turned off. Close is a
// no-op.
package nocache
