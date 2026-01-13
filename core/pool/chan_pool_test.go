package pool

import (
	"bytes"
	"context"
	"sync"
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

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				buf := pool.Get()
				buf.WriteString("test")
				pool.Put(buf)
			}
		}()
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
