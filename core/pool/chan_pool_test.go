package pool

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestChanPool(t *testing.T) {
	newBuf := func() *bytes.Buffer {
		return new(bytes.Buffer)
	}
	resetBuf := func(b *bytes.Buffer) *bytes.Buffer {
		b.Reset()
		return b
	}

	pool := NewChanPool(2,
		WithNew(newBuf),
		WithReset(resetBuf),
	)

	// Get two objects
	buf1 := pool.Get()
	buf2 := pool.Get()
	if buf1 == nil || buf2 == nil {
		t.Fatal("expected non-nil buffers")
	}

	// Put them back
	if !pool.Put(buf1) {
		t.Fatal("expected first Put to return true")
	}
	if !pool.Put(buf2) {
		t.Fatal("expected second Put to return true")
	}

	// Pool is full, next Put should return false
	buf3 := newBuf()
	if pool.Put(buf3) {
		t.Fatal("expected Put to return false when pool is full")
	}
}

func TestChanPoolAccept(t *testing.T) {
	accept := func(b *bytes.Buffer) bool {
		// Only accept buffers with capacity less than 100
		return b.Cap() < 100
	}

	pool := NewChanPool(2,
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
		WithAccept(accept),
	)

	// Normal buffer can be put back
	buf := pool.Get()
	if !pool.Put(buf) {
		t.Fatal("expected Put to return true for normal buffer")
	}

	// Buffer with large capacity due to writing too much data cannot be put back
	buf.WriteString(string(make([]byte, 101)))
	if pool.Put(buf) {
		t.Fatal("expected Put to return false for large buffer")
	}
}

func TestChanPoolGetContext(t *testing.T) {
	pool := NewChanPool(1,
		WithNew(func() string { return "new" }),
	)

	// First put an object to the pool
	pool.Put("pooled")

	// Getting from pool should return immediately
	obj, ok := pool.GetContext(context.Background())
	if !ok || obj != "pooled" {
		t.Fatalf("expected ok=true, obj='pooled'; got ok=%v, obj=%q", ok, obj)
	}

	// Second Get will create a new object (because newFunc exists)
	obj, ok = pool.GetContext(context.Background())
	if !ok || obj != "new" {
		t.Fatalf("expected ok=true, obj='new'; got ok=%v, obj=%q", ok, obj)
	}

	// Pool without newFunc, timeout test
	poolNoNew := NewChanPool[string](1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, ok = poolNoNew.GetContext(ctx)
	if ok {
		t.Fatalf("expected ok=false after context timeout")
	}
}

func TestChanPoolConcurrent(t *testing.T) {
	pool := NewChanPool(10,
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
		WithReset(func(b *bytes.Buffer) *bytes.Buffer {
			b.Reset()
			return b
		}),
	)

	var wg sync.WaitGroup
	const goroutines = 100
	const iterations = 1000

	for range goroutines {
		wg.Go(func() {
			for range iterations {
				buf := pool.Get()
				buf.WriteString("test")
				pool.Put(buf)
			}
		})
	}
	wg.Wait()
}

func TestChanPoolLenCap(t *testing.T) {
	pool := NewChanPool[int](5,
		WithNew(func() int { return 42 }),
	)

	if pool.Cap() != 5 {
		t.Fatalf("expected Cap=5, got %d", pool.Cap())
	}

	if pool.Len() != 0 {
		t.Fatalf("expected Len=0, got %d", pool.Len())
	}

	// Put 3 objects
	pool.Put(1)
	pool.Put(2)
	pool.Put(3)

	if pool.Len() != 3 {
		t.Fatalf("expected Len=3, got %d", pool.Len())
	}

	// Get 1 object
	pool.Get()
	if pool.Len() != 2 {
		t.Fatalf("expected Len=2, got %d", pool.Len())
	}
}

func TestChanPoolGetContextBlocking(t *testing.T) {
	pool := NewChanPool[string](1)

	// Put object later in another goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		pool.Put("delayed")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	obj, ok := pool.GetContext(ctx)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("expected ok=true")
	}
	if obj != "delayed" {
		t.Fatalf("expected 'delayed', got %q", obj)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("expected to wait at least 40ms, waited %v", elapsed)
	}
}

func TestChanPoolZeroSize(t *testing.T) {
	pool := NewChanPool[int](0,
		WithNew(func() int { return 1 }),
	)

	// Should be corrected to 1
	if pool.Cap() != 1 {
		t.Fatalf("expected Cap=1, got %d", pool.Cap())
	}

	poolNeg := NewChanPool[int](-5,
		WithNew(func() int { return 1 }),
	)
	if poolNeg.Cap() != 1 {
		t.Fatalf("expected Cap=1, got %d", poolNeg.Cap())
	}
}

func TestChanPoolWithoutNew(t *testing.T) {
	pool := NewChanPool[*bytes.Buffer](2)

	// Pool is empty, no New function, should return nil
	buf := pool.Get()
	if buf != nil {
		t.Fatalf("expected nil, got %v", buf)
	}
}

func BenchmarkChanPoolGet(b *testing.B) {
	pool := NewChanPool(10,
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Get()
	}
}

func BenchmarkChanPoolGetPut(b *testing.B) {
	pool := NewChanPool(10,
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
		WithReset(func(buf *bytes.Buffer) *bytes.Buffer {
			buf.Reset()
			return buf
		}),
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkChanPoolConcurrent(b *testing.B) {
	pool := NewChanPool(100,
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
		WithReset(func(buf *bytes.Buffer) *bytes.Buffer {
			buf.Reset()
			return buf
		}),
	)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf.WriteString("test")
			pool.Put(buf)
		}
	})
}

func TestChanPoolClose(t *testing.T) {
	closeCount := 0
	pool := NewChanPool(2,
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
		WithClose(func(b *bytes.Buffer) {
			closeCount++
		}),
	)

	// Put two objects
	pool.Put(new(bytes.Buffer))
	pool.Put(new(bytes.Buffer))

	// Close the pool
	pool.Close()

	if closeCount != 2 {
		t.Fatalf("expected closeFn to be called 2 times, got %d", closeCount)
	}

	if !pool.IsClosed() {
		t.Fatal("expected pool to be closed")
	}

	// Operations after close should fail gracefully
	obj, ok := pool.GetContext(context.Background())
	if ok {
		t.Fatal("expected GetContext to fail after close")
	}
	if obj != nil {
		t.Fatalf("expected nil object after close, got %v", obj)
	}

	// Double close should be safe
	pool.Close()
}

func TestChanPoolGetContextWithAllowCreate(t *testing.T) {
	t.Run("AllowCreate=true (default)", func(t *testing.T) {
		pool := NewChanPool(1,
			WithNew(func() string { return "new" }),
		)

		// Pool is empty, but newFn exists and allowCreate is true
		obj, ok := pool.GetContext(context.Background())
		if !ok {
			t.Fatal("expected ok=true")
		}
		if obj != "new" {
			t.Fatalf("expected 'new', got %q", obj)
		}
	})

	t.Run("AllowCreate=false (force wait)", func(t *testing.T) {
		pool := NewChanPool(1,
			WithNew(func() string { return "new" }),
			WithAllowCreate[string](false),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Pool is empty, allowCreate is false, should wait and timeout
		start := time.Now()
		_, ok := pool.GetContext(ctx)
		elapsed := time.Since(start)

		if ok {
			t.Fatal("expected ok=false after timeout")
		}
		if elapsed < 40*time.Millisecond {
			t.Fatalf("expected to wait at least 40ms, waited %v", elapsed)
		}
	})

	t.Run("AllowCreate=false with delayed Put", func(t *testing.T) {
		pool := NewChanPool(1,
			WithNew(func() string { return "new" }),
			WithAllowCreate[string](false),
		)

		// Put object later
		go func() {
			time.Sleep(30 * time.Millisecond)
			pool.Put("delayed")
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		start := time.Now()
		obj, ok := pool.GetContext(ctx)
		elapsed := time.Since(start)

		if !ok {
			t.Fatal("expected ok=true")
		}
		if obj != "delayed" {
			t.Fatalf("expected 'delayed', got %q", obj)
		}
		if elapsed < 20*time.Millisecond {
			t.Fatalf("expected to wait at least 20ms, waited %v", elapsed)
		}
	})
}

// TestChanPoolConcurrentPutClose exercises the Put/Close race (BUG-05): a Put
// that overlaps with Close must never send on a closed channel. Run with -race.
func TestChanPoolConcurrentPutClose(t *testing.T) {
	for range 200 {
		pool := NewChanPool[int](8)

		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func(v int) {
				defer wg.Done()
				pool.Put(v)
			}(i)
		}

		pool.Close()
		wg.Wait()

		if !pool.IsClosed() {
			t.Fatal("pool should report closed")
		}
	}
}

// TestChanPoolGetContextClosedWhileBlocked covers BUG-13: a GetContext blocked
// on an empty pool must return (zero, false) when Close wakes it, never a zero
// value with ok=true.
func TestChanPoolGetContextClosedWhileBlocked(t *testing.T) {
	pool := NewChanPool[int](1, WithAllowCreate[int](false))

	type result struct {
		obj int
		ok  bool
	}
	resCh := make(chan result, 1)
	go func() {
		obj, ok := pool.GetContext(context.Background())
		resCh <- result{obj: obj, ok: ok}
	}()

	// Give the goroutine time to block on the empty pool, then close.
	time.Sleep(20 * time.Millisecond)
	pool.Close()

	select {
	case res := <-resCh:
		if res.ok {
			t.Fatalf("expected ok=false after close, got ok=true obj=%d", res.obj)
		}
	case <-time.After(time.Second):
		t.Fatal("GetContext did not return after Close")
	}
}

// TestChanPoolGetClosedWithNew verifies that Get() on a closed pool falls back to
// newFn if configured, or returns the zero value of T when exhausted.
func TestChanPoolGetClosedWithNew(t *testing.T) {
	t.Run("closed empty pool with newFn returns created object", func(t *testing.T) {
		createdCount := 0
		pool := NewChanPool[string](2, WithNew(func() string {
			createdCount++
			return "created"
		}))

		pool.Close()

		got := pool.Get()
		if got != "created" {
			t.Fatalf("expected 'created', got %q", got)
		}
		if createdCount != 1 {
			t.Fatalf("expected createdCount=1, got %d", createdCount)
		}
	})

	t.Run("closed empty pool without newFn returns zero value", func(t *testing.T) {
		pool := NewChanPool[int](2)
		pool.Close()

		got := pool.Get()
		if got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})

	t.Run("closed pool with leftover items returns items then falls back to newFn", func(t *testing.T) {
		createdCount := 0
		pool := NewChanPool[string](2, WithNew(func() string {
			createdCount++
			return "new-item"
		}))

		pool.Put("item-1")
		pool.Close()

		// First Get returns the leftover item in the channel
		got1 := pool.Get()
		if got1 != "item-1" {
			t.Fatalf("expected 'item-1', got %q", got1)
		}

		// Second Get receives on closed channel, falls back to newFn
		got2 := pool.Get()
		if got2 != "new-item" {
			t.Fatalf("expected 'new-item', got %q", got2)
		}
		if createdCount != 1 {
			t.Fatalf("expected createdCount=1, got %d", createdCount)
		}
	})
}

// TestChanPoolStress_ConcurrentGetPutClose executes heavy concurrent Get, GetContext,
// Put, and Close across 60+ goroutines under -race over many iterations.
func TestChanPoolStress_ConcurrentGetPutClose(t *testing.T) {
	const iterations = 50

	for iter := range iterations {
		var (
			createdCount atomic.Int64
			closedCount  atomic.Int64
			resetCount   atomic.Int64
			acceptCount  atomic.Int64
		)

		poolCap := (iter % 10) + 1
		p := NewChanPool(poolCap,
			WithNew(func() string {
				c := createdCount.Add(1)
				return fmt.Sprintf("val-%d", c)
			}),
			WithReset(func(s string) string {
				resetCount.Add(1)
				return s
			}),
			WithAccept(func(s string) bool {
				acceptCount.Add(1)
				return len(s) > 0
			}),
			WithClose(func(s string) {
				closedCount.Add(1)
			}),
		)

		var wg sync.WaitGroup
		startSignal := make(chan struct{})

		// 20 Get & Put workers
		for range 20 {
			wg.Go(func() {
				<-startSignal
				for range 50 {
					v := p.Get()
					if v != "" {
						p.Put(v)
					}
				}
			})
		}

		// 20 GetContext & Put workers
		for i := range 20 {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-startSignal
				for j := range 50 {
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(j%5)*time.Millisecond)
					v, ok := p.GetContext(ctx)
					cancel()
					if ok && v != "" {
						p.Put(v)
					}
				}
			}(i)
		}

		// 20 Put & Len/Cap/IsClosed inspectors
		for i := range 20 {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				<-startSignal
				for j := range 50 {
					p.Put(fmt.Sprintf("item-%d-%d", workerID, j))
					_ = p.Len()
					_ = p.Cap()
					_ = p.IsClosed()
				}
			}(i)
		}

		// 1 Close worker triggered after short random delay
		wg.Go(func() {
			<-startSignal
			time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
			p.Close()
			// Double close test
			p.Close()
		})

		close(startSignal)
		wg.Wait()

		if !p.IsClosed() {
			t.Fatalf("iteration %d: pool should be closed", iter)
		}

		// Post-close verification
		if p.Put("after-close") {
			t.Fatalf("iteration %d: Put after Close must return false", iter)
		}

		ctx := context.Background()
		_, ok := p.GetContext(ctx)
		if ok {
			t.Fatalf("iteration %d: GetContext after Close must return ok=false", iter)
		}
	}
}

// TestChanPoolStress_MassiveConcurrency50Plus runs 100 concurrent workers on a buffer pool.
func TestChanPoolStress_MassiveConcurrency50Plus(t *testing.T) {
	p := NewChanPool(20,
		WithNew(func() *bytes.Buffer {
			return new(bytes.Buffer)
		}),
		WithReset(func(b *bytes.Buffer) *bytes.Buffer {
			b.Reset()
			return b
		}),
		WithAccept(func(b *bytes.Buffer) bool {
			return b != nil
		}),
	)

	const goroutines = 100
	const opsPerGoroutine = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range opsPerGoroutine {
				var buf *bytes.Buffer
				if j%2 == 0 {
					buf = p.Get()
				} else {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
					var ok bool
					buf, ok = p.GetContext(ctx)
					cancel()
					if !ok || buf == nil {
						buf = new(bytes.Buffer)
					}
				}

				if buf == nil {
					t.Errorf("worker %d: unexpected nil buffer", id)
					return
				}

				buf.WriteString("sample data")
				_ = p.Put(buf)
			}
		}(i)
	}

	wg.Wait()
	p.Close()
}

// TestChanPoolStress_GetAfterCloseSemantics exhaustively verifies Get() post-Close behavior.
func TestChanPoolStress_GetAfterCloseSemantics(t *testing.T) {
	t.Run("drained closed pool with WithNew returns new instances repeatedly", func(t *testing.T) {
		var counter atomic.Int32
		p := NewChanPool(5, WithNew(func() int32 {
			return counter.Add(1)
		}))

		p.Put(100)
		p.Put(200)
		p.Close() // channel closed, no closeFn so items 100 and 200 remain in channel

		// First two Get() calls retrieve leftover items
		if v := p.Get(); v != 100 {
			t.Fatalf("expected 100, got %d", v)
		}
		if v := p.Get(); v != 200 {
			t.Fatalf("expected 200, got %d", v)
		}

		// Channel is now drained and closed; next Get() calls must invoke WithNew
		for i := int32(1); i <= 10; i++ {
			if v := p.Get(); v != i {
				t.Fatalf("expected WithNew call %d, got %d", i, v)
			}
		}
	})

	t.Run("drained closed pool without WithNew returns zero value", func(t *testing.T) {
		p := NewChanPool[string](5)
		p.Put("leftover")
		p.Close()

		if v := p.Get(); v != "leftover" {
			t.Fatalf("expected 'leftover', got %q", v)
		}

		// Now empty and closed without WithNew
		for range 5 {
			if v := p.Get(); v != "" {
				t.Fatalf("expected empty string zero value, got %q", v)
			}
		}
	})

	t.Run("closed with WithClose drains items then WithNew triggers", func(t *testing.T) {
		var drained []string
		p := NewChanPool(5,
			WithNew(func() string { return "brand-new" }),
			WithClose(func(s string) {
				drained = append(drained, s)
			}),
		)

		p.Put("item1")
		p.Put("item2")
		p.Close() // WithClose drains "item1" and "item2"

		if len(drained) != 2 {
			t.Fatalf("expected 2 drained items, got %d", len(drained))
		}

		// Channel is drained by Close(); Get() should immediately return new item
		if v := p.Get(); v != "brand-new" {
			t.Fatalf("expected 'brand-new', got %q", v)
		}
	})
}
