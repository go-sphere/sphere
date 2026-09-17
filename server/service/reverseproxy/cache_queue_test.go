package reverseproxy

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestCacheQueueConcurrentWriteAndClose(t *testing.T) {
	for range 100 {
		q := newCacheQueue()
		var wg sync.WaitGroup
		wg.Go(func() { _, _ = q.Write([]byte("body")) })
		wg.Go(func() { q.CloseWithError(nil) })
		wg.Wait()
		if _, err := io.ReadAll(q); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCacheQueueWriteNeverBlocks pins the property the proxy depends on: a
// cache.Save that does not read — a stalled backend — must not stop the copy
// loop, so a write past the queue's depth is reported instead of blocking.
func TestCacheQueueWriteNeverBlocks(t *testing.T) {
	q := newCacheQueue()
	chunk := bytes.Repeat([]byte("x"), 32*1024)

	done := make(chan error, 1)
	go func() {
		for i := 0; i < cacheQueueDepth; i++ {
			if _, err := q.Write(chunk); err != nil {
				done <- err
				return
			}
		}
		_, err := q.Write(chunk)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, errCacheQueueFull) {
			t.Fatalf("write past the depth: err = %v, want errCacheQueueFull", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Write blocked with no reader; a stalled cache would stall the client")
	}
}

// TestCacheQueueDeliversInOrderThenEOF pins that closing the stream cleanly
// still delivers every queued chunk before the reader sees io.EOF, so a normal
// response is saved in full.
func TestCacheQueueDeliversInOrderThenEOF(t *testing.T) {
	q := newCacheQueue()
	chunks := [][]byte{[]byte("first"), []byte("second"), []byte("third")}
	for _, c := range chunks {
		if _, err := q.Write(c); err != nil {
			t.Fatalf("Write(%q) error = %v", c, err)
		}
	}
	q.CloseWithError(nil)

	got, err := io.ReadAll(q)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if want := bytes.Join(chunks, nil); !bytes.Equal(got, want) {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

// TestCacheQueueCloseWithErrorSurfacesError pins that the copy loop's failure
// reaches Save, which reports it through the configured error handler, and that
// the write buffer is copied rather than aliased.
func TestCacheQueueCloseWithErrorSurfacesError(t *testing.T) {
	q := newCacheQueue()
	sentinel := errors.New("client went away")

	buf := []byte("payload")
	if _, err := q.Write(buf); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	// The copy loop reuses its read buffer; overwriting it must not change what
	// the reader already received.
	copy(buf, "XXXXXXX")
	q.CloseWithError(sentinel)

	got, err := io.ReadAll(q)
	if !errors.Is(err, sentinel) {
		t.Fatalf("ReadAll() error = %v, want %v", err, sentinel)
	}
	if string(got) != "payload" {
		t.Fatalf("body = %q, want %q", got, "payload")
	}
}

// TestCacheQueueCloseIsIdempotent pins that the several close sites in the copy
// loop (client failure, cache overflow, defer) can fire in any combination
// without panicking on a double close of the channel.
func TestCacheQueueCloseIsIdempotent(t *testing.T) {
	q := newCacheQueue()
	first := errors.New("first close wins")
	q.CloseWithError(first)
	q.CloseWithError(nil)

	if _, err := q.Write([]byte("late")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Write after close: err = %v, want io.ErrClosedPipe", err)
	}
	if _, err := io.ReadAll(q); !errors.Is(err, first) {
		t.Fatalf("ReadAll() error = %v, want the first close's error", err)
	}
}

// TestCacheQueueReadHonoursSmallBuffers pins that a reader with a buffer
// smaller than a queued chunk still receives the whole chunk across reads.
func TestCacheQueueReadHonoursSmallBuffers(t *testing.T) {
	q := newCacheQueue()
	if _, err := q.Write([]byte("0123456789")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	q.CloseWithError(nil)

	var got []byte
	buf := make([]byte, 3)
	for {
		n, err := q.Read(buf)
		got = append(got, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
	}
	if string(got) != "0123456789" {
		t.Fatalf("body = %q, want %q", got, "0123456789")
	}
}
