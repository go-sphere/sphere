package reverseproxy

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

// cacheQueueDepth is how many copy-loop chunks a cacheQueue holds before it
// reports overflow. The copy loop reads 32 KiB at a time, so the queue buffers
// at most 512 KiB of a response that the cache is not keeping up with. Dropping
// the cache beyond that point is deliberate: the alternative is to hold the
// bytes, and with them the upstream response, in memory for as long as the
// backend stalls.
const cacheQueueDepth = 16

// errCacheQueueFull is reported by cacheQueue.Write once the queue holds
// cacheQueueDepth chunks. It is the caller's signal to abandon the cache and
// keep serving the client.
var errCacheQueueFull = errors.New("reverseproxy: cache queue is full")

// cacheQueue is the hand-off between the goroutine copying the upstream body
// to the client and the goroutine saving that body to the cache.
//
// It replaces the io.Pipe the pair used to share. A pipe write returns only
// once the reader has consumed the bytes, and that reader is cache.Save, which
// writes to the configured backend between reads: a backend that stalls — a
// remote cache on a congested link, a lock held by another writer — therefore
// blocked the copy loop, which (a) throttled the client's download to the
// cache's write speed and (b) if the backend never consumed at all, parked the
// response goroutine for good, since the loop never got back around to the
// client write that would have noticed the disconnect. The cache is
// best-effort; the client must not pay for it.
//
// Write enqueues a copy of the chunk and returns immediately. A full queue is
// reported as errCacheQueueFull rather than blocking. Read drains the queue in
// order and then reports the writer's terminal error, or io.EOF when the stream
// ended cleanly. One goroutine writes and one reads; CloseWithError may be
// called from either and more than once, and any chunks already queued are
// still delivered.
type cacheQueue struct {
	ch chan []byte

	mu     sync.Mutex
	err    error
	closed bool

	// cur is the chunk Read is currently draining. Only Read touches it.
	cur []byte
}

func newCacheQueue() *cacheQueue {
	return &cacheQueue{ch: make(chan []byte, cacheQueueDepth)}
}

// Write enqueues a copy of p. It never blocks: once the queue is full it
// reports errCacheQueueFull so the caller can drop the cache instead of the
// client's stream. A write after CloseWithError reports io.ErrClosedPipe.
func (q *cacheQueue) Write(p []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return 0, io.ErrClosedPipe
	}
	// p is the copy loop's reusable read buffer, so the bytes have to be
	// copied out before they are queued.
	chunk := bytes.Clone(p)
	select {
	case q.ch <- chunk:
		return len(p), nil
	default:
		return 0, errCacheQueueFull
	}
}

// CloseWithError ends the stream once. Queued chunks are still readable;
// afterwards Read reports err, or io.EOF when err is nil.
func (q *cacheQueue) CloseWithError(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.err = err
	close(q.ch)
}

// Read drains queued chunks in order, blocking only until the writer queues the
// next one or ends the stream.
func (q *cacheQueue) Read(p []byte) (int, error) {
	for len(q.cur) == 0 {
		chunk, ok := <-q.ch
		if !ok {
			q.mu.Lock()
			err := q.err
			q.mu.Unlock()
			if err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		q.cur = chunk
	}
	n := copy(p, q.cur)
	q.cur = q.cur[n:]
	return n, nil
}
