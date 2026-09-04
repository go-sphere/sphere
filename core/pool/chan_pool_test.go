package pool

import (
	"bytes"
	"context"
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
	var resets atomic.Int64
	pool := NewChanPool(10,
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
		WithReset(func(b *bytes.Buffer) *bytes.Buffer {
			resets.Add(1)
			b.Reset()
			return b
		}),
	)

	var wg sync.WaitGroup
	const goroutines = 16
	const iterations = 100

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
	if got, want := resets.Load(), int64(goroutines*iterations); got != want {
		t.Fatalf("reset calls = %d, want %d", got, want)
	}
	if got := pool.Len(); got == 0 || got > pool.Cap() {
		t.Fatalf("retained objects = %d, want within [1, %d]", got, pool.Cap())
	}
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
	type result struct {
		obj string
		ok  bool
	}
	resultCh := make(chan result, 1)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	go func() {
		obj, ok := pool.GetContext(ctx)
		resultCh <- result{obj: obj, ok: ok}
	}()

	select {
	case got := <-resultCh:
		t.Fatalf("GetContext returned before Put: %+v", got)
	case <-time.After(20 * time.Millisecond):
	}
	if !pool.Put("delayed") {
		t.Fatal("Put delayed object failed")
	}
	got := <-resultCh

	if !got.ok {
		t.Fatal("expected ok=true")
	}
	if got.obj != "delayed" {
		t.Fatalf("expected 'delayed', got %q", got.obj)
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

		ctx, cancel := context.WithCancel(t.Context())
		result := make(chan bool, 1)
		go func() {
			_, ok := pool.GetContext(ctx)
			result <- ok
		}()
		select {
		case ok := <-result:
			t.Fatalf("GetContext returned before cancellation: ok=%v", ok)
		case <-time.After(20 * time.Millisecond):
		}
		cancel()
		if ok := <-result; ok {
			t.Fatal("expected ok=false after cancellation")
		}
	})

	t.Run("AllowCreate=false with delayed Put", func(t *testing.T) {
		pool := NewChanPool(1,
			WithNew(func() string { return "new" }),
			WithAllowCreate[string](false),
		)

		type result struct {
			obj string
			ok  bool
		}
		resultCh := make(chan result, 1)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		go func() {
			obj, ok := pool.GetContext(ctx)
			resultCh <- result{obj: obj, ok: ok}
		}()
		select {
		case got := <-resultCh:
			t.Fatalf("GetContext returned before Put: %+v", got)
		case <-time.After(20 * time.Millisecond):
		}
		if !pool.Put("delayed") {
			t.Fatal("Put delayed object failed")
		}
		got := <-resultCh
		if !got.ok {
			t.Fatal("expected ok=true")
		}
		if got.obj != "delayed" {
			t.Fatalf("expected 'delayed', got %q", got.obj)
		}
	})
}

// TestChanPoolConcurrentPutClose exercises the Put/Close race (BUG-05): a Put
// that overlaps with Close must never send on a closed channel. Run with -race.
func TestChanPoolConcurrentPutClose(t *testing.T) {
	for range 32 {
		pool := NewChanPool[int](8)

		var accepted atomic.Int64
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				if pool.Put(i + 1) {
					accepted.Add(1)
				}
			})
		}

		pool.Close()
		wg.Wait()

		if !pool.IsClosed() {
			t.Fatal("pool should report closed")
		}
		var retained int64
		for pool.Get() != 0 {
			retained++
		}
		if got := accepted.Load(); got != retained {
			t.Fatalf("accepted puts = %d, retained objects = %d", got, retained)
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

	select {
	case res := <-resCh:
		t.Fatalf("GetContext returned before Close: %+v", res)
	case <-time.After(20 * time.Millisecond):
	}
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
