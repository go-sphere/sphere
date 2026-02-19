package pool

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSyncPool(t *testing.T) {
	newBuf := func() *bytes.Buffer {
		return new(bytes.Buffer)
	}
	resetBuf := func(b *bytes.Buffer) *bytes.Buffer {
		b.Reset()
		return b
	}

	pool := NewSyncPool(
		WithNew(newBuf),
		WithReset(resetBuf),
	)

	// Get a new object
	buf1 := pool.Get()
	if buf1 == nil {
		t.Fatal("expected non-nil buffer")
	}

	// Write data
	buf1.WriteString("hello")
	if buf1.String() != "hello" {
		t.Fatalf("expected 'hello', got %q", buf1.String())
	}

	// Put back to pool (will be reset)
	if !pool.Put(buf1) {
		t.Fatal("expected Put to return true")
	}

	// Get the same object (already reset)
	buf2 := pool.Get()
	if buf2.String() != "" {
		t.Fatalf("expected empty buffer after reset, got %q", buf2.String())
	}
}

func TestSyncPoolWithoutReset(t *testing.T) {
	pool := NewSyncPool[int](
		WithNew(func() int { return 42 }),
	)

	val := pool.Get()
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}

	if !pool.Put(val) {
		t.Fatal("expected Put to return true")
	}
}

func TestSyncPoolWithoutNew(t *testing.T) {
	pool := NewSyncPool[*bytes.Buffer]()

	// Without New function, should return nil
	buf := pool.Get()
	if buf != nil {
		t.Fatalf("expected nil, got %v", buf)
	}
}

func TestSyncPoolConcurrent(t *testing.T) {
	pool := NewSyncPool(
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

func TestSyncPoolNewFuncCalled(t *testing.T) {
	var callCount atomic.Int32

	pool := NewSyncPool(
		WithNew(func() *bytes.Buffer {
			callCount.Add(1)
			return new(bytes.Buffer)
		}),
	)

	// First Get should call New
	_ = pool.Get()
	if callCount.Load() != 1 {
		t.Fatalf("expected New to be called once, called %d times", callCount.Load())
	}

	// Without putting back, next Get should call New again
	_ = pool.Get()
	if callCount.Load() != 2 {
		t.Fatalf("expected New to be called twice, called %d times", callCount.Load())
	}
}

func BenchmarkSyncPoolGet(b *testing.B) {
	pool := NewSyncPool(
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Get()
	}
}

func BenchmarkSyncPoolGetPut(b *testing.B) {
	pool := NewSyncPool(
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

func BenchmarkSyncPoolConcurrent(b *testing.B) {
	pool := NewSyncPool(
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

func TestSyncPoolAccept(t *testing.T) {
	accept := func(b *bytes.Buffer) bool {
		// Only accept buffers with capacity less than 100
		return b.Cap() < 100
	}

	pool := NewSyncPool(
		WithNew(func() *bytes.Buffer { return new(bytes.Buffer) }),
		WithAccept(accept),
		WithReset(func(buf *bytes.Buffer) *bytes.Buffer {
			buf.Reset()
			return buf
		}),
	)

	// Normal buffer can be put back
	buf := pool.Get()
	if !pool.Put(buf) {
		t.Fatal("expected Put to return true for normal buffer")
	}

	// Get it back to verify it was accepted
	buf1 := pool.Get()
	if buf1 == nil {
		t.Fatal("expected non-nil buffer")
	}

	// Buffer with large capacity due to writing too much data cannot be put back
	buf1.WriteString(string(make([]byte, 101)))
	if pool.Put(buf1) {
		t.Fatal("expected Put to return false for large buffer")
	}

	// The rejected buffer should not be in the pool
	// Getting a new buffer should create a fresh one (via New function)
	buf2 := pool.Get()
	if buf2 == nil {
		t.Fatal("expected non-nil buffer")
	}
	// Fresh buffer should have small capacity
	if buf2.Cap() >= 100 {
		t.Fatalf("expected buffer with cap < 100, got cap=%d", buf2.Cap())
	}
}
