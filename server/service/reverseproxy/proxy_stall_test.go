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

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage/local"
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

// ctxReadCache models an object-store backend: it consumes the body in a
// goroutine, as a multipart uploader would, and aborts as soon as its context
// is done. It reports the error Save returned on saved.
type ctxReadCache struct {
	Cache
	saved chan error
}

func (c *ctxReadCache) Save(ctx context.Context, _ string, _ http.Header, r io.Reader) error {
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, r)
		done <- err
	}()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}
	c.saved <- err
	return err
}

// TestReverseProxySaveTimeoutIsIdle pins that the save timeout bounds the time
// the body goes without making progress, not the transfer as a whole: a slow
// but steady download longer than the timeout is still saved, while an upstream
// that stops sending mid-body is abandoned once the timeout passes.
func TestReverseProxySaveTimeoutIsIdle(t *testing.T) {
	const (
		saveTimeout = 400 * time.Millisecond
		chunks      = 20
		chunkSize   = 512
	)
	tests := []struct {
		name    string
		stall   bool // upstream stops after the first chunk and never finishes
		wantErr bool
	}{
		{name: "steady transfer longer than timeout is saved"},
		{name: "stalled upstream is abandoned after timeout", stall: true, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache := &ctxReadCache{Cache: setupTestCache(t), saved: make(chan error, 1)}

			unblock := make(chan struct{})
			defer close(unblock)
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(http.StatusOK)
				for range chunks {
					_, _ = w.Write(bytes.Repeat([]byte("x"), chunkSize))
					w.(http.Flusher).Flush()
					if tc.stall {
						select {
						case <-unblock:
						case <-r.Context().Done():
						}
						return
					}
					time.Sleep(50 * time.Millisecond)
				}
			}))
			defer backend.Close()

			backendURL, err := url.Parse(backend.URL)
			if err != nil {
				t.Fatalf("Parse backend URL: %v", err)
			}
			proxy, err := CreateCacheReverseProxy(cache,
				WithTargetURL(backendURL),
				WithSaveTimeout(saveTimeout),
			)
			if err != nil {
				t.Fatalf("CreateCacheReverseProxy: %v", err)
			}
			frontend := httptest.NewServer(http.HandlerFunc(ServeCacheReverseProxy(cache, proxy)))
			defer frontend.Close()

			start := time.Now()
			resp, err := frontend.Client().Get(frontend.URL + "/idle")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if tc.stall {
				buf := make([]byte, chunkSize)
				if _, err := io.ReadFull(resp.Body, buf); err != nil {
					t.Fatalf("read first chunk: %v", err)
				}
			} else {
				body, err := io.ReadAll(resp.Body)
				if err != nil || len(body) != chunks*chunkSize {
					t.Fatalf("client read %d bytes, err %v; want %d", len(body), err, chunks*chunkSize)
				}
				if elapsed := time.Since(start); elapsed <= saveTimeout {
					t.Fatalf("transfer took %v, want longer than the save timeout %v", elapsed, saveTimeout)
				}
			}

			select {
			case err := <-cache.saved:
				if (err != nil) != tc.wantErr {
					t.Fatalf("Save error = %v, want error: %v", err, tc.wantErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Save did not finish")
			}
		})
	}
}

// recordingSaveCache reports the error of every Save on saved.
type recordingSaveCache struct {
	Cache
	saved chan error
}

func (c *recordingSaveCache) Save(ctx context.Context, key string, header http.Header, r io.Reader) error {
	err := c.Cache.Save(ctx, key, header, r)
	c.saved <- err
	return err
}

// TestReverseProxySaveTimeoutReleasesBlockedStorageRead pins that the idle
// timeout frees a Save that is parked reading the body inside a driver that
// ignores its context mid-copy (local storage does), so the key can be cached
// again afterwards.
func TestReverseProxySaveTimeoutReleasesBlockedStorageRead(t *testing.T) {
	store, err := local.NewClient(local.Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("local.NewClient: %v", err)
	}
	headerCache := memory.NewByteCache()
	t.Cleanup(func() { _ = headerCache.Close() })
	cache := &recordingSaveCache{Cache: NewByteCache(headerCache, store), saved: make(chan error, 4)}

	unblock := make(chan struct{})
	defer close(unblock)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1024))
		w.(http.Flusher).Flush()
		select {
		case <-unblock:
		case <-r.Context().Done():
		}
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Parse backend URL: %v", err)
	}
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL), WithSaveTimeout(200*time.Millisecond))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}
	frontend := httptest.NewServer(http.HandlerFunc(ServeCacheReverseProxy(cache, proxy)))
	defer frontend.Close()

	for round := range 2 {
		resp, err := frontend.Client().Get(frontend.URL + "/stall")
		if err != nil {
			t.Fatalf("round %d GET: %v", round, err)
		}
		if _, err := io.ReadFull(resp.Body, make([]byte, 1024)); err != nil {
			t.Fatalf("round %d read: %v", round, err)
		}
		select {
		case err := <-cache.saved:
			if !errors.Is(err, errSaveIdle) {
				t.Fatalf("round %d Save error = %v, want errSaveIdle", round, err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("round %d: Save still blocked after the idle timeout", round)
		}
		_ = resp.Body.Close()
	}
}
