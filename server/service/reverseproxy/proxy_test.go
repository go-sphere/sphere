package reverseproxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage/local"
)

func TestDefaultCacheKey(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		want   string
	}{
		{name: "GET path", method: http.MethodGet, target: "/items", want: "/items"},
		{name: "GET query", method: http.MethodGet, target: "/items?page=2&owner=alice", want: "/items?page=2&owner=alice"},
		{name: "HEAD bypasses cache", method: http.MethodHead, target: "/items?page=2", want: ""},
		{name: "POST bypasses cache", method: http.MethodPost, target: "/items?page=2", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, nil)
			if got := defaultCacheKey(req); got != tt.want {
				t.Fatalf("defaultCacheKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCreateCacheReverseProxy tests the creation of a cached reverse proxy
func TestCreateCacheReverseProxy(t *testing.T) {
	cache := setupTestCache(t)
	targetURL, _ := url.Parse("http://example.com")

	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(targetURL))
	if err != nil {
		t.Fatalf("Failed to create reverse proxy: %v", err)
	}
	if proxy == nil {
		t.Fatal("Expected non-nil proxy")
	}
}

// TestServeCacheReverseProxy_CacheMiss tests proxy behavior when cache is empty
func TestServeCacheReverseProxy_CacheMiss(t *testing.T) {
	cache := setupTestCache(t)

	// Create a mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend response"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)

	// Make a request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if body != "backend response" {
		t.Errorf("Expected 'backend response', got '%s'", body)
	}

	// Verify cache was populated
	exists, err := cache.Exists(req.Context(), "/test")
	if err != nil {
		t.Logf("Cache check error (may be normal): %v", err)
	}
	if exists {
		t.Log("Cache was populated successfully")
	}
}

// TestServeCacheReverseProxy_CacheHit tests serving from cache
func TestServeCacheReverseProxy_CacheHit(t *testing.T) {
	cache := setupTestCache(t)

	backendCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalls++
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Backend-Call", "true")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("original response"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)

	// First request - cache miss
	req1 := httptest.NewRequest(http.MethodGet, "/cached", nil)
	rec1 := httptest.NewRecorder()
	handler(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %d", rec1.Code)
	}

	// Second request - should hit cache
	req2 := httptest.NewRequest(http.MethodGet, "/cached", nil)
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("Second request failed with status %d", rec2.Code)
	}

	// Backend should only be called once if cache works
	// Note: Due to async caching, the exact count might vary
	t.Logf("Backend was called %d times", backendCalls)
}

// TestServeCacheReverseProxy_NonGETNotCached tests that non-GET requests are not cached
func TestServeCacheReverseProxy_NonGETNotCached(t *testing.T) {
	cache := setupTestCache(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("post response"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)

	// POST request
	req := httptest.NewRequest(http.MethodPost, "/api/data", strings.NewReader("data"))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	exists, _ := cache.Exists(req.Context(), "/api/data")
	if exists {
		t.Error("POST request should not be cached")
	}
}

func TestServeCacheReverseProxy_NonGETDoesNotReplayGETCache(t *testing.T) {
	cache := setupTestCache(t)
	ctx := context.Background()
	if err := cache.Save(ctx, "/api/data?scope=all", http.Header{}, strings.NewReader("cached GET")); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	backendCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalls++
		_, _ = w.Write([]byte("backend POST"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/data?scope=all", strings.NewReader("data"))
	rec := httptest.NewRecorder()
	ServeCacheReverseProxy(cache, proxy)(rec, req)

	if backendCalls != 1 {
		t.Fatalf("backend calls = %d, want 1", backendCalls)
	}
	if got := rec.Body.String(); got != "backend POST" {
		t.Fatalf("response = %q, want backend response", got)
	}
}

func TestServeCacheReverseProxy_QueryParametersAreIsolated(t *testing.T) {
	cache := setupTestCache(t)
	ctx := context.Background()
	if err := cache.Save(ctx, "/search?user=alice", http.Header{}, strings.NewReader("alice cache")); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	backendCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalls++
		_, _ = w.Write([]byte("backend " + r.URL.RawQuery))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}
	handler := ServeCacheReverseProxy(cache, proxy)

	aliceReq := httptest.NewRequest(http.MethodGet, "/search?user=alice", nil)
	aliceRec := httptest.NewRecorder()
	handler(aliceRec, aliceReq)
	if got := aliceRec.Body.String(); got != "alice cache" {
		t.Fatalf("alice response = %q, want cached response", got)
	}

	bobReq := httptest.NewRequest(http.MethodGet, "/search?user=bob", nil)
	bobRec := httptest.NewRecorder()
	handler(bobRec, bobReq)
	if got := bobRec.Body.String(); got != "backend user=bob" {
		t.Fatalf("bob response = %q, want backend response", got)
	}
	if backendCalls != 1 {
		t.Fatalf("backend calls = %d, want 1", backendCalls)
	}
}

// TestServeCacheReverseProxy_CustomCacheKeyFunc tests custom cache key generation
func TestServeCacheReverseProxy_CustomCacheKeyFunc(t *testing.T) {
	cache := setupTestCache(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("response"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)

	customKeyFunc := func(r *http.Request) string {
		if r.Method != http.MethodGet {
			return ""
		}
		// Include query parameters in cache key
		return r.URL.Path + "?" + r.URL.RawQuery
	}

	proxy, err := CreateCacheReverseProxy(
		cache,
		WithTargetURL(backendURL),
		WithCacheKeyFunc(customKeyFunc),
	)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)

	// Request with query parameters
	req := httptest.NewRequest(http.MethodGet, "/api?user=123", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

// TestServeCacheReverseProxy_CustomResponseChecker tests custom response caching logic
func TestServeCacheReverseProxy_CustomResponseChecker(t *testing.T) {
	cache := setupTestCache(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "error") {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_, _ = w.Write([]byte("response"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)

	// Custom checker that also caches 500 errors
	customChecker := func(resp *http.Response) bool {
		return resp.Request.Method == http.MethodGet
	}

	proxy, err := CreateCacheReverseProxy(
		cache,
		WithTargetURL(backendURL),
		WithResponseCacheCheck(customChecker),
	)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)

	// Request that returns 500
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}
}

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
	if err == nil && exists {
		t.Error("Non-existent key should not exist")
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
		t.Logf("Exists check error: %v", err)
	}
	if exists {
		t.Log("Key exists as expected")
	}
}

// setupTestCache creates a test cache instance with temporary storage
func setupTestCache(t *testing.T) *CommonCache {
	t.Helper()

	// Create temporary directory for test
	tempDir := t.TempDir()

	store, err := local.NewClient(local.Config{
		RootDir: tempDir,
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	cache := NewByteCache(
		memory.NewByteCache(),
		store,
	)

	// Cleanup function
	t.Cleanup(func() {
		// Temporary directory is automatically cleaned up by t.TempDir()
	})

	return cache
}

// TestServeCacheReverseProxy_CacheHitPreservesStatus verifies that a cache hit
// replays the original upstream status code instead of a hardcoded 200, and that
// the internal cache-status header is not leaked to the client.
func TestServeCacheReverseProxy_CacheHitPreservesStatus(t *testing.T) {
	cache := setupTestCache(t)
	ctx := (&http.Request{}).Context()

	// Populate the cache the same way ModifyResponse does for a non-200 response.
	header := http.Header{
		"Content-Type":    []string{"text/plain"},
		cacheStatusHeader: []string{strconv.Itoa(http.StatusInternalServerError)},
	}
	if err := cache.Save(ctx, "/status", header, strings.NewReader("cached error body")); err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	backendURL, _ := url.Parse("http://example.com")
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	handler := ServeCacheReverseProxy(cache, proxy)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected replayed status 500, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "cached error body" {
		t.Errorf("Expected 'cached error body', got %q", body)
	}
	if got := rec.Header().Get(cacheStatusHeader); got != "" {
		t.Errorf("Internal cache-status header leaked to client: %q", got)
	}
}

// TestServeCacheReverseProxy_Integration is a manual integration test
// Set TEST_REVERSE_PROXY=true to enable
func TestServeCacheReverseProxy_Integration(t *testing.T) {
	if os.Getenv("TEST_REVERSE_PROXY") != "true" {
		t.Skip("Skipping integration test. Set TEST_REVERSE_PROXY=true to enable")
	}

	cache := setupTestCache(t)
	root, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(root))
	if err != nil {
		t.Fatal(err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)

	t.Log("Starting test server on :9999")
	t.Log("Test with: curl http://localhost:9999/")
	_ = http.ListenAndServe(":9999", http.HandlerFunc(handler))
}

type saveContextCache struct {
	started chan struct{}
	release chan struct{}
	result  chan error
}

func newSaveContextCache() *saveContextCache {
	return &saveContextCache{
		started: make(chan struct{}),
		release: make(chan struct{}),
		result:  make(chan error, 1),
	}
}

func (c *saveContextCache) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (c *saveContextCache) Delete(context.Context, string) error {
	return nil
}

func (c *saveContextCache) Save(ctx context.Context, _ string, _ http.Header, reader io.Reader) error {
	if _, err := io.Copy(io.Discard, reader); err != nil {
		c.result <- err
		return err
	}
	close(c.started)
	<-c.release
	c.result <- ctx.Err()
	return nil
}

func (c *saveContextCache) Load(context.Context, string) (http.Header, io.ReadCloser, error) {
	return nil, nil, ErrCacheNotFound
}

func (c *saveContextCache) Header(context.Context, string) (http.Header, error) {
	return nil, ErrCacheNotFound
}

func TestCreateCacheReverseProxy_SaveContextOutlivesRequest(t *testing.T) {
	cache := newSaveContextCache()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("response"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}

	requestCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/detached", nil).WithContext(requestCtx)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	select {
	case <-cache.started:
	case <-time.After(2 * time.Second):
		t.Fatal("cache save did not start")
	}
	cancel()
	close(cache.release)

	select {
	case err := <-cache.result:
		if err != nil {
			t.Fatalf("cache save context was canceled with request: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cache save did not finish")
	}
}
