// Package pool offers two generic object pools that share one [Option] set.
//
// [SyncPool] (from [NewSyncPool]) wraps sync.Pool: unbounded, GC-eligible,
// no Close. Use it for short-lived buffers. [ChanPool] (from [NewChanPool])
// is a buffered channel with a fixed capacity: use it for connections or
// file handles that must be closed, and call [ChanPool.Close] when done.
// [WithClose] and [WithAllowCreate] are ChanPool-only; SyncPool ignores them.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/core/pool"
//
//	p := pool.NewChanPool(4,
//		pool.WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
//		pool.WithReset(func(b *bytes.Buffer) *bytes.Buffer { b.Reset(); return b }),
//	)
//	defer p.Close()
//
//	buf := p.Get()
//	buf.WriteString("hello")
//	p.Put(buf) // false when rejected by Accept, the pool is full, or closed
//
// Put runs Accept then Reset; a rejected object is neither reset, pooled, nor
// passed to Close. GetContext on a closed ChanPool returns (zero, false) even
// if items remain in the channel. After Close, Get still drains leftover items
// when no Close callback was set.
package pool
