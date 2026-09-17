package reverseproxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

// stalledSaveCache models a cache backend that has stopped making progress: it
// reads nothing from the body it is handed and does not return until Released,
// ignoring its context entirely. A wedged connection or a lock held by another
// writer behaves this way.
type stalledSaveCache struct {
	Cache
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *stalledSaveCache) Save(context.Context, string, http.Header, io.Reader) error {
	c.once.Do(func() { close(c.started) })
	<-c.release
	return errors.New("stalled save released")
}

// TestReverseProxyStalledCacheDoesNotStallClient pins that the client's
// download does not inherit the cache's pace. The copy loop used to write each
// chunk straight into an io.Pipe that cache.Save reads, so a Save that stopped
// reading blocked the loop after the first 32 KiB: the client kept a 200 and
// then received nothing for as long as the backend stayed wedged, however long
// that was (the save timeout only bounds a Save that honours its context).
// Bodies are handed over a bounded queue now, so the client gets all of it and
// the cache is the thing that is dropped.
//
// The payload is larger than the queue's depth so the overflow path is taken,
// and the save timeout is set far beyond the client's own timeout so that a
// passing test cannot be the timeout doing the work.
func TestReverseProxyStalledCacheDoesNotStallClient(t *testing.T) {
	stalled := &stalledSaveCache{
		Cache:   setupTestCache(t),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	defer close(stalled.release)

	payload := bytes.Repeat([]byte("0123456789ABCDEF"), (cacheQueueDepth+4)*2048)
	expectedHash := sha256.Sum256(payload)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Parse backend URL: %v", err)
	}

	proxy, err := CreateCacheReverseProxy(stalled,
		WithTargetURL(backendURL),
		WithSaveTimeout(time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}

	frontend := httptest.NewServer(http.HandlerFunc(ServeCacheReverseProxy(stalled, proxy)))
	defer frontend.Close()

	// A client timeout turns a regression into a failure instead of a hang.
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(frontend.URL + "/stalled-cache")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("client body read failed: %v (a stalled cache must not stall the client)", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(body) != len(payload) {
		t.Fatalf("client received %d bytes, want %d", len(body), len(payload))
	}
	if sha256.Sum256(body) != expectedHash {
		t.Fatal("client received a corrupted body")
	}

	// The save must really have been started and still be parked, otherwise the
	// test would not have exercised the stall it claims to.
	select {
	case <-stalled.started:
	case <-time.After(5 * time.Second):
		t.Fatal("cache Save was never called; the test did not exercise a stalled save")
	}
	select {
	case <-stalled.release:
		t.Fatal("Save returned before the test released it")
	default:
	}
}
