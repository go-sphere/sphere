package reverseproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/storage"
)

// TestCommonCache_SaveAndLoad tests the cache save and load operations
func TestCommonCache_SaveAndLoad(t *testing.T) {
	cache := setupTestCache(t)

	ctx := (&http.Request{}).Context()
	header := http.Header{
		"Content-Type": []string{"text/plain"},
		"X-Custom":     []string{"value"},
	}
	body := strings.NewReader("test content")

	// Save to cache
	err := cache.Save(ctx, "test-key", header, body)
	if err != nil {
		t.Fatalf("Failed to save to cache: %v", err)
	}

	// Load from cache
	loadedHeader, loadedBody, err := cache.Load(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to load from cache: %v", err)
	}
	defer func() {
		if closer, ok := loadedBody.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	// Verify header
	if loadedHeader.Get("Content-Type") != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", loadedHeader.Get("Content-Type"))
	}
	if loadedHeader.Get("X-Custom") != "value" {
		t.Errorf("Expected X-Custom 'value', got '%s'", loadedHeader.Get("X-Custom"))
	}

	// Verify body
	loadedContent, err := io.ReadAll(loadedBody)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}
	if string(loadedContent) != "test content" {
		t.Errorf("Expected 'test content', got '%s'", string(loadedContent))
	}
}

func TestCommonCache_SaveRootPath(t *testing.T) {
	cache := setupTestCache(t)
	ctx := t.Context()
	header := http.Header{"Content-Type": []string{"text/plain"}}

	if err := cache.Save(ctx, "/", header, strings.NewReader("homepage")); err != nil {
		t.Fatalf("Save GET /: %v", err)
	}
	loadedHeader, loadedBody, err := cache.Load(ctx, "/")
	if err != nil {
		t.Fatalf("Load GET /: %v", err)
	}
	defer ignoreCloseError(loadedBody.Close)
	if loadedHeader.Get("Content-Type") != "text/plain" {
		t.Errorf("Content-Type = %q", loadedHeader.Get("Content-Type"))
	}
	got, err := io.ReadAll(loadedBody)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "homepage" {
		t.Errorf("body = %q, want homepage", got)
	}

	if err := cache.Save(ctx, "/foo", header, strings.NewReader("without slash")); err != nil {
		t.Fatalf("Save /foo: %v", err)
	}
	if err := cache.Save(ctx, "/foo/", header, strings.NewReader("with slash")); err != nil {
		t.Fatalf("Save /foo/: %v", err)
	}
	_, otherBody, err := cache.Load(ctx, "/foo")
	if err != nil {
		t.Fatalf("Load /foo: %v", err)
	}
	defer ignoreCloseError(otherBody.Close)
	got, err = io.ReadAll(otherBody)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "without slash" {
		t.Errorf("/foo body clobbered by /foo/: %q", got)
	}
	_, slashBody, err := cache.Load(ctx, "/foo/")
	if err != nil {
		t.Fatalf("Load /foo/: %v", err)
	}
	defer ignoreCloseError(slashBody.Close)
	got, err = io.ReadAll(slashBody)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "with slash" {
		t.Errorf("/foo/ body clobbered by /foo: %q", got)
	}
}

// TestCommonCache_Delete tests cache deletion
func TestCommonCache_Delete(t *testing.T) {
	cache := setupTestCache(t)

	ctx := (&http.Request{}).Context()
	header := http.Header{"Content-Type": []string{"text/plain"}}
	body := strings.NewReader("delete test")

	// Save to cache
	err := cache.Save(ctx, "delete-key", header, body)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Delete from cache
	err = cache.Delete(ctx, "delete-key")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	// Verify deletion
	_, _, err = cache.Load(ctx, "delete-key")
	if err == nil {
		t.Error("Expected error when loading deleted key, got nil")
	}
}

// TestCommonCache_Exists tests cache existence check
func TestCommonCache_Exists(t *testing.T) {
	cache := setupTestCache(t)

	ctx := (&http.Request{}).Context()

	// Check non-existent key
	exists, err := cache.Exists(ctx, "non-existent")
	if !errors.Is(err, ErrCacheNotFound) {
		t.Fatalf("Exists(non-existent) error = %v, want %v", err, ErrCacheNotFound)
	}
	if exists {
		t.Fatal("non-existent key exists")
	}

	// Save and check
	header := http.Header{"Content-Type": []string{"text/plain"}}
	body := strings.NewReader("exists test")
	err = cache.Save(ctx, "exists-key", header, body)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	exists, err = cache.Exists(ctx, "exists-key")
	if err != nil {
		t.Fatalf("Exists(saved key): %v", err)
	}
	if !exists {
		t.Fatal("saved key does not exist")
	}
}

// TestCommonCache_ExpiredHeaderDoesNotOrphanBody pins that once a header blob
// is gone (expired or evicted by the header cache), the body object it pointed
// at does not stay in storage forever: the miss reclaims it.
func TestCommonCache_ExpiredHeaderDoesNotOrphanBody(t *testing.T) {
	for _, tc := range []struct {
		name string
		miss func(ctx context.Context, c *CommonCache, key string) error
	}{
		{name: "Load", miss: func(ctx context.Context, c *CommonCache, key string) error {
			_, _, err := c.Load(ctx, key)
			return err
		}},
		{name: "Exists", miss: func(ctx context.Context, c *CommonCache, key string) error {
			_, err := c.Exists(ctx, key)
			return err
		}},
		{name: "Header", miss: func(ctx context.Context, c *CommonCache, key string) error {
			_, err := c.Header(ctx, key)
			return err
		}},
		{name: "Delete", miss: func(ctx context.Context, c *CommonCache, key string) error {
			return c.Delete(ctx, key)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := setupTestCache(t)
			ctx := t.Context()
			const key = "/expiring"

			if err := c.Save(ctx, key, http.Header{}, strings.NewReader("body")); err != nil {
				t.Fatalf("Save: %v", err)
			}
			// Simulate the header cache expiring the entry on its own.
			if err := c.cache.Del(ctx, key); err != nil {
				t.Fatalf("expire header: %v", err)
			}
			if err := tc.miss(ctx, c, key); !errors.Is(err, ErrCacheNotFound) {
				t.Fatalf("%s after expiry = %v, want ErrCacheNotFound", tc.name, err)
			}
			waitOrphanCleanups(t, c)
			exists, err := c.storage.IsFileExists(ctx, storageObjectName(key))
			if err != nil {
				t.Fatalf("IsFileExists: %v", err)
			}
			if exists {
				t.Fatal("body object outlived its expired header")
			}
		})
	}
}

// uploadHookStorage runs afterUpload once a body upload has completed, i.e.
// inside Save between storing the body and storing its header blob.
type uploadHookStorage struct {
	storage.Storage
	afterUpload func()
}

func (s *uploadHookStorage) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	stored, err := s.Storage.UploadFile(ctx, file, key)
	if err == nil && s.afterUpload != nil {
		s.afterUpload()
	}
	return stored, err
}

// TestCommonCache_MissDuringSaveKeepsBody pins that the orphan cleanup run by
// a miss does not delete the body a concurrent Save of the same key has just
// uploaded but not yet published: the entry must be complete once Save
// returns.
func TestCommonCache_MissDuringSaveKeepsBody(t *testing.T) {
	c := setupTestCache(t)
	ctx := t.Context()
	const key = "/in-flight"

	c.storage = &uploadHookStorage{Storage: c.storage, afterUpload: func() {
		if _, err := c.Exists(ctx, key); !errors.Is(err, ErrCacheNotFound) {
			t.Errorf("Exists during Save = %v, want ErrCacheNotFound", err)
		}
	}}
	if err := c.Save(ctx, key, http.Header{}, strings.NewReader("body")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, body, err := c.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	defer ignoreCloseError(body.Close)
	if got, err := io.ReadAll(body); err != nil || string(got) != "body" {
		t.Fatalf("body = %q, %v; want %q", got, err, "body")
	}
}

// blockingDeleteStorage counts DeleteFile calls and, when release is set,
// holds each one until release is closed.
type blockingDeleteStorage struct {
	storage.Storage
	deletes atomic.Int32
	release chan struct{}
}

func (s *blockingDeleteStorage) DeleteFile(ctx context.Context, key string) error {
	s.deletes.Add(1)
	if s.release != nil {
		<-s.release
	}
	return s.Storage.DeleteFile(ctx, key)
}

// TestCommonCache_MissDoesNotWaitForCleanup pins that the orphan-body cleanup
// a miss triggers runs off the caller's path: a slow storage delete must not
// delay the miss.
func TestCommonCache_MissDoesNotWaitForCleanup(t *testing.T) {
	c := setupTestCache(t)
	store := &blockingDeleteStorage{Storage: c.storage, release: make(chan struct{})}
	c.storage = store

	done := make(chan error, 1)
	go func() {
		_, _, err := c.Load(context.Background(), "/missing")
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrCacheNotFound) {
			t.Errorf("Load = %v, want ErrCacheNotFound", err)
		}
	case <-time.After(time.Second):
		t.Error("Load waited for the storage delete")
	}
	close(store.release)
	waitOrphanCleanups(t, c)
	if got := store.deletes.Load(); got != 1 {
		t.Fatalf("DeleteFile calls = %d, want 1", got)
	}
}

// TestServeCacheReverseProxy_UncacheableMissCleansUpOncePerWindow pins that a
// URL that is never stored does not cost a storage delete on every request:
// repeated misses clean the key up once per window.
func TestServeCacheReverseProxy_UncacheableMissCleansUpOncePerWindow(t *testing.T) {
	c := setupTestCache(t)
	store := &blockingDeleteStorage{Storage: c.storage}
	c.storage = store

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("private"))
	}))
	defer backend.Close()
	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(c, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}
	handler := ServeCacheReverseProxy(c, proxy)

	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		waitOrphanCleanups(t, c)
	}
	if got := store.deletes.Load(); got != 1 {
		t.Fatalf("DeleteFile calls for 10 misses = %d, want 1", got)
	}
}

// waitOrphanCleanups waits until no background orphan cleanup of c is running.
func waitOrphanCleanups(t *testing.T, c *CommonCache) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		c.cleanups.mu.Lock()
		running := c.cleanups.running
		c.cleanups.mu.Unlock()
		if running == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d orphan cleanups still running", running)
		}
		time.Sleep(time.Millisecond)
	}
}

// setOrphanCleanupLimits overrides the cleanup limits for one test. Tests that
// call it must not run in parallel.
func setOrphanCleanupLimits(t *testing.T, window time.Duration, maxKeys, maxRunning int) {
	t.Helper()
	oldWindow, oldKeys, oldRunning := orphanCleanupWindow, orphanCleanupMaxKeys, orphanCleanupMaxRunning
	orphanCleanupWindow, orphanCleanupMaxKeys, orphanCleanupMaxRunning = window, maxKeys, maxRunning
	t.Cleanup(func() {
		orphanCleanupWindow, orphanCleanupMaxKeys, orphanCleanupMaxRunning = oldWindow, oldKeys, oldRunning
	})
}

// TestCommonCache_OrphanCleanupWindowExpires pins that the per-key limit is a
// window, not a one-off: once it has passed, the next miss cleans up again.
func TestCommonCache_OrphanCleanupWindowExpires(t *testing.T) {
	setOrphanCleanupLimits(t, 50*time.Millisecond, 16, 4)
	c := setupTestCache(t)
	store := &blockingDeleteStorage{Storage: c.storage}
	c.storage = store

	miss := func() {
		if _, err := c.Exists(context.Background(), "/missing"); !errors.Is(err, ErrCacheNotFound) {
			t.Fatalf("Exists = %v, want ErrCacheNotFound", err)
		}
		waitOrphanCleanups(t, c)
	}
	miss()
	miss()
	if got := store.deletes.Load(); got != 1 {
		t.Fatalf("DeleteFile calls within the window = %d, want 1", got)
	}
	time.Sleep(60 * time.Millisecond)
	miss()
	if got := store.deletes.Load(); got != 2 {
		t.Fatalf("DeleteFile calls after the window = %d, want 2", got)
	}
}

// TestCommonCache_OrphanCleanupConcurrencyIsBounded pins that misses beyond the
// running limit skip their cleanup instead of queueing it, and that a skipped
// key is not remembered, so a later miss can still clean it up.
func TestCommonCache_OrphanCleanupConcurrencyIsBounded(t *testing.T) {
	const maxRunning = 3
	setOrphanCleanupLimits(t, time.Minute, 64, maxRunning)
	c := setupTestCache(t)
	store := &blockingDeleteStorage{Storage: c.storage, release: make(chan struct{})}
	c.storage = store

	for i := range maxRunning + 5 {
		if _, err := c.Exists(context.Background(), fmt.Sprintf("/missing-%d", i)); !errors.Is(err, ErrCacheNotFound) {
			t.Fatalf("Exists = %v, want ErrCacheNotFound", err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for store.deletes.Load() < maxRunning && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	c.cleanups.mu.Lock()
	running, remembered := c.cleanups.running, len(c.cleanups.recent)
	c.cleanups.mu.Unlock()
	if running != maxRunning || remembered != maxRunning {
		t.Fatalf("running = %d, remembered = %d, want %d each", running, remembered, maxRunning)
	}
	close(store.release)
	waitOrphanCleanups(t, c)
	if got := store.deletes.Load(); got != maxRunning {
		t.Fatalf("DeleteFile calls = %d, want %d", got, maxRunning)
	}

	// A key skipped for lack of capacity is cleaned up by its next miss.
	if _, err := c.Exists(context.Background(), fmt.Sprintf("/missing-%d", maxRunning)); !errors.Is(err, ErrCacheNotFound) {
		t.Fatalf("Exists = %v, want ErrCacheNotFound", err)
	}
	waitOrphanCleanups(t, c)
	if got := store.deletes.Load(); got != maxRunning+1 {
		t.Fatalf("DeleteFile calls after retry = %d, want %d", got, maxRunning+1)
	}
}

// TestCommonCache_OrphanCleanupRecordIsBounded pins that the record of
// recently cleaned keys stays within its cap, dropping the oldest keys first.
func TestCommonCache_OrphanCleanupRecordIsBounded(t *testing.T) {
	const maxKeys = 4
	setOrphanCleanupLimits(t, time.Minute, maxKeys, 64)
	c := setupTestCache(t)

	for i := range 10 {
		if _, err := c.Exists(context.Background(), fmt.Sprintf("/missing-%d", i)); !errors.Is(err, ErrCacheNotFound) {
			t.Fatalf("Exists = %v, want ErrCacheNotFound", err)
		}
	}
	waitOrphanCleanups(t, c)
	c.cleanups.mu.Lock()
	defer c.cleanups.mu.Unlock()
	if len(c.cleanups.recent) != maxKeys || len(c.cleanups.order) != maxKeys {
		t.Fatalf("record holds %d keys (%d ordered), want %d", len(c.cleanups.recent), len(c.cleanups.order), maxKeys)
	}
	for i := 10 - maxKeys; i < 10; i++ {
		if _, ok := c.cleanups.recent[fmt.Sprintf("/missing-%d", i)]; !ok {
			t.Fatalf("newest key /missing-%d was dropped", i)
		}
	}
}
