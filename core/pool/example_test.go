package pool_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/go-sphere/sphere/core/pool"
)

// Bounded pool with reset-on-Put and a destructor run by Close.
func ExampleNewChanPool() {
	p := pool.NewChanPool(4,
		pool.WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
		pool.WithReset(func(b *bytes.Buffer) *bytes.Buffer { b.Reset(); return b }),
		pool.WithClose(func(*bytes.Buffer) { fmt.Println("closed one buffer") }),
	)
	defer p.Close()

	buf := p.Get()
	buf.WriteString("hello")
	fmt.Println(buf.String())
	fmt.Println("retained:", p.Put(buf), "len:", p.Len())
	fmt.Println("reset:", p.Get().Len() == 0)
	p.Put(new(bytes.Buffer))
	// Output:
	// hello
	// retained: true len: 1
	// reset: true
	// closed one buffer
}

// With WithAllowCreate(false), GetContext waits for a Put instead of calling
// New; it reports false when ctx ends first.
func ExampleChanPool_GetContext() {
	p := pool.NewChanPool(1,
		pool.WithNew(func() int { return 42 }),
		pool.WithAllowCreate[int](false),
	)
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // stands in for a deadline that has passed
	_, ok := p.GetContext(ctx)
	fmt.Println("got from empty pool:", ok)

	p.Put(7)
	v, ok := p.GetContext(context.Background())
	fmt.Println(v, ok)
	// Output:
	// got from empty pool: false
	// 7 true
}

// WithAccept rejects objects that should not be reused, such as oversized
// buffers.
func ExampleNewSyncPool() {
	p := pool.NewSyncPool(
		pool.WithNew(func() []byte { return make([]byte, 0, 64) }),
		pool.WithAccept(func(b []byte) bool { return cap(b) <= 1024 }),
		pool.WithReset(func(b []byte) []byte { return b[:0] }),
	)

	b := p.Get()
	b = append(b, "data"...)
	fmt.Println(p.Put(b))
	fmt.Println(p.Put(make([]byte, 0, 4096)))
	// Output:
	// true
	// false
}
