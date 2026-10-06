// Package memory is the ristretto-backed in-process cache.Cache driver.
//
// Use it for a high-throughput, process-local cache. Entry points:
// [NewMemoryCache] for typed values (cost 1 per item), [NewByteCache] for
// []byte values costed by length, [NewMemoryCacheWithCost] for a custom cost
// function, and [NewMemoryCacheWithRistretto] to wrap a configured ristretto
// instance.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/cache/memory"
//
//	c := memory.NewMemoryCache[string]()
//	defer c.Close()
//
//	if err := c.Set(ctx, "greeting", "hello"); err != nil {
//		return err
//	}
//	v, found, err := c.Get(ctx, "greeting")
//
// # Behavior
//
//   - Writes wait for ristretto's buffers (so a Get after Set sees the value)
//     unless [Cache.SetAllowAsyncWrites] enables async writes.
//   - Ristretto may drop a write under load or by admission policy; that is
//     reported as success, as caches are allowed to lose entries.
//   - No cache.KeyLister: nscache.NSCache.DelAll over this driver returns
//     cache.ErrNotSupported unless another lister is in the stack.
//   - Same-key mutations and GetDel are serialized by 128 striped mutexes, so
//     GetDel returns an entry as found to at most one caller.
//   - After Close, every method returns cache.ErrClosed; this is the only
//     driver that does so. UpdateMaxCost on a closed cache is a silent no-op.
//   - The ctx argument of CRUD methods is unused.
//   - []byte values are copied on Set and Get; other value types are stored
//     as given.
//
// [NewMemoryCache], [NewMemoryCacheWithCost], and [NewByteCache] own the
// ristretto instance and Close it. [NewMemoryCacheWithRistretto] does not.
package memory
