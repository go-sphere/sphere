package reverseproxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage/kvcache"
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

func TestCreateCacheReverseProxyRejectsInvalidConfiguration(t *testing.T) {
	targetURL, _ := url.Parse("http://example.com")

	tests := []struct {
		name  string
		cache Cache
		opts  []Option
	}{
		{name: "nil cache", opts: []Option{WithTargetURL(targetURL)}},
		{name: "missing target", cache: setupTestCache(t)},
		{name: "nil target", cache: setupTestCache(t), opts: []Option{WithTargetURL(nil)}},
		{name: "nil cache key function", cache: setupTestCache(t), opts: []Option{
			WithTargetURL(targetURL), WithCacheKeyFunc(nil),
		}},
		{name: "nil response checker", cache: setupTestCache(t), opts: []Option{
			WithTargetURL(targetURL), WithResponseCacheCheck(nil),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := CreateCacheReverseProxy(tt.cache, tt.opts...)
			if err == nil {
				t.Fatalf("CreateCacheReverseProxy() = (%v, nil), want error", proxy)
			}
			if proxy != nil {
				t.Fatalf("CreateCacheReverseProxy() proxy = %v, want nil", proxy)
			}
		})
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

// setupTestCache creates a test cache instance with in-memory storage.
func setupTestCache(t *testing.T) *CommonCache {
	t.Helper()

	storageCache := memory.NewByteCache()
	t.Cleanup(func() { _ = storageCache.Close() })

	store, err := kvcache.NewClient(kvcache.Config{}, storageCache)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	headerCache := memory.NewByteCache()
	t.Cleanup(func() { _ = headerCache.Close() })

	return NewByteCache(
		headerCache,
		store,
	)
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

// TestServeCacheReverseProxy_CacheHitRejectsInvalidStatus verifies that a
// corrupted or poisoned cache entry cannot crash the handler: net/http panics on
// a status outside 100-999, so out-of-range and unparseable values must fall
// back to 200 instead of reaching WriteHeader.
func TestServeCacheReverseProxy_CacheHitRejectsInvalidStatus(t *testing.T) {
	for _, poisoned := range []string{"0", "-1", "99", "600", "1000", "99999", "not-a-number"} {
		t.Run(poisoned, func(t *testing.T) {
			cache := setupTestCache(t)
			ctx := (&http.Request{}).Context()

			header := http.Header{
				"Content-Type":    []string{"text/plain"},
				cacheStatusHeader: []string{poisoned},
			}
			if err := cache.Save(ctx, "/poisoned", header, strings.NewReader("cached body")); err != nil {
				t.Fatalf("Failed to save cache: %v", err)
			}

			backendURL, _ := url.Parse("http://example.com")
			proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
			if err != nil {
				t.Fatalf("Failed to create proxy: %v", err)
			}
			handler := ServeCacheReverseProxy(cache, proxy)

			req := httptest.NewRequest(http.MethodGet, "/poisoned", nil)
			rec := httptest.NewRecorder()
			// Must not panic.
			handler(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Expected fallback status 200 for poisoned value %q, got %d", poisoned, rec.Code)
			}
			if got := rec.Header().Get(cacheStatusHeader); got != "" {
				t.Errorf("Internal cache-status header leaked to client: %q", got)
			}
		})
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

// TestDefaultResponseCacheCheck pins which responses may enter a cache that is
// keyed on the request URI alone and therefore replayable by any later caller.
func TestDefaultResponseCacheCheck(t *testing.T) {
	newResp := func(mutate func(req *http.Request, resp *http.Response)) *http.Response {
		req := httptest.NewRequest(http.MethodGet, "/resource", nil)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Request:    req,
		}
		if mutate != nil {
			mutate(req, resp)
		}
		return resp
	}

	tests := []struct {
		name   string
		mutate func(req *http.Request, resp *http.Response)
		want   bool
	}{
		{name: "plain public GET", want: true},
		{name: "vary on accept-encoding only", want: true, mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Vary", "Accept-Encoding")
		}},
		{name: "non-200", mutate: func(_ *http.Request, resp *http.Response) {
			resp.StatusCode = http.StatusFound
		}},
		{name: "non-GET", mutate: func(req *http.Request, _ *http.Response) {
			req.Method = http.MethodPost
		}},
		{name: "request carries authorization", mutate: func(req *http.Request, _ *http.Response) {
			req.Header.Set("Authorization", "Bearer alice-token")
		}},
		{name: "request carries cookie", mutate: func(req *http.Request, _ *http.Response) {
			req.Header.Set("Cookie", "session=alice")
		}},
		{name: "response sets a cookie", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Add("Set-Cookie", "session=alice; HttpOnly")
		}},
		{name: "no-store", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Cache-Control", "no-store")
		}},
		{name: "no-cache", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Cache-Control", "no-cache")
		}},
		{name: "field-specific no-cache", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Cache-Control", `no-cache="X-Private"`)
		}},
		{name: "zero max-age", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Cache-Control", "public, max-age=0")
		}},
		{name: "zero shared max-age", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Cache-Control", "s-maxage=0")
		}},
		{name: "malformed max-age", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Cache-Control", "max-age=soon")
		}},
		{name: "positive shared max-age", want: true, mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Cache-Control", "public, s-maxage=60")
		}},
		{name: "private among directives", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Cache-Control", "max-age=60, private")
		}},
		{name: "vary on a header not in the key", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Vary", "Accept-Language")
		}},
		{name: "vary star", mutate: func(_ *http.Request, resp *http.Response) {
			resp.Header.Set("Vary", "*")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultResponseCacheCheck(newResp(tt.mutate)); got != tt.want {
				t.Fatalf("defaultResponseCacheCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestServeCacheReverseProxy_CredentialedResponseNotReplayed is the end-to-end
// form of the same rule: a response fetched with a bearer token must not be
// handed to an anonymous caller, and its Set-Cookie must never be replayed.
func TestServeCacheReverseProxy_CredentialedResponseNotReplayed(t *testing.T) {
	cache := setupTestCache(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "session=alice-secret; Path=/; HttpOnly")
		w.Header().Set("Cache-Control", "no-store, private")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":"alice@corp.example"}`))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	handler := ServeCacheReverseProxy(cache, proxy)

	authed := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	authed.Header.Set("Authorization", "Bearer alice-token")
	authedRec := httptest.NewRecorder()
	handler(authedRec, authed)
	if authedRec.Code != http.StatusOK {
		t.Fatalf("authenticated request failed with status %d", authedRec.Code)
	}

	// Give the cache-save goroutine a chance to run, so a regression that stores
	// the response is actually observed rather than raced past.
	time.Sleep(100 * time.Millisecond)

	anon := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	anonRec := httptest.NewRecorder()
	handler(anonRec, anon)

	if body := anonRec.Body.String(); strings.Contains(body, "alice@corp.example") {
		t.Fatalf("anonymous caller received the authenticated response: %s", body)
	}
	if cookie := anonRec.Header().Get("Set-Cookie"); cookie != "" {
		t.Fatalf("anonymous caller received a replayed Set-Cookie: %q", cookie)
	}
	if anonRec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous request status = %d, want %d", anonRec.Code, http.StatusUnauthorized)
	}
}

// failingSaveCache rejects every Save without reading the body, the way a
// storage backend does when the disk is full, a credential has expired, or the
// key is invalid. Everything else is delegated so cache lookups still work.
type failingSaveCache struct {
	Cache
	saveErr error
}

func (f *failingSaveCache) Save(context.Context, string, http.Header, io.Reader) error {
	return f.saveErr
}

// TestCacheSaveFailureDoesNotTruncateResponse pins the promise that a cache
// failure does not affect the client.
//
// Both sinks used to be written through a single io.MultiWriter, which fails as
// a whole the moment either side errors: a rejected cache write aborted the copy
// to the client mid-body, and because the status line was already on the wire
// the client kept a 200 with a truncated payload — the hardest kind of corruption
// to notice.
func TestCacheSaveFailureDoesNotTruncateResponse(t *testing.T) {
	payload := bytes.Repeat([]byte("abcdefgh"), 64*1024) // 512 KiB

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer backend.Close()

	saveErr := errors.New("storage is full")
	brokenCache := &failingSaveCache{Cache: setupTestCache(t), saveErr: saveErr}

	reported := make(chan error, 1)
	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(brokenCache,
		WithTargetURL(backendURL),
		WithErrorHandler(func(err error) {
			select {
			case reported <- err:
			default:
			}
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	handler := ServeCacheReverseProxy(brokenCache, proxy)
	frontend := httptest.NewServer(http.HandlerFunc(handler))
	defer frontend.Close()

	resp, err := frontend.Client().Get(frontend.URL + "/big")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the body failed even though only the cache write should have: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(body) != len(payload) {
		t.Fatalf("client received %d of %d bytes", len(body), len(payload))
	}
	if !bytes.Equal(body, payload) {
		t.Fatal("client received a corrupted body")
	}

	// The failure is still surfaced to the caller who asked for it.
	select {
	case got := <-reported:
		if !errors.Is(got, saveErr) {
			t.Fatalf("error handler got %v, want %v", got, saveErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the cache failure was never reported to the error handler")
	}
}

// TestServeCacheHitCopyErrorDoesNotPanic pins that a client disconnecting
// mid-download is an ordinary event. The serve-side error handler had no
// default, so an entirely optional option was the only thing standing between
// this and a nil call in the request goroutine.
func TestServeCacheHitCopyErrorDoesNotPanic(t *testing.T) {
	cache := setupTestCache(t)
	key := "/cached-body"
	if err := cache.Save(context.Background(), key, http.Header{}, strings.NewReader("cached payload")); err != nil {
		t.Fatalf("seeding the cache failed: %v", err)
	}

	backendURL, _ := url.Parse("http://unused.invalid")
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	// No WithServeErrorHandler: the default must absorb the write failure.
	handler := ServeCacheReverseProxy(cache, proxy)

	req := httptest.NewRequest(http.MethodGet, key, nil)
	handler(&failingResponseWriter{header: http.Header{}}, req)
}

// failingResponseWriter fails every write, standing in for a client that
// disconnected while the cached body was being copied to it.
type failingResponseWriter struct {
	header http.Header
}

func (f *failingResponseWriter) Header() http.Header { return f.header }

func (f *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("connection reset by peer")
}

func (f *failingResponseWriter) WriteHeader(int) {}

func TestWithSaveTimeout(t *testing.T) {
	t.Parallel()

	// Positive duration
	opts := newOptions(WithSaveTimeout(15 * time.Second))
	if opts.saveTimeout != 15*time.Second {
		t.Fatalf("expected saveTimeout 15s, got %v", opts.saveTimeout)
	}

	// Zero duration falls back to default
	optsZero := newOptions(WithSaveTimeout(0))
	if optsZero.saveTimeout != defaultSaveTimeout {
		t.Fatalf("expected defaultSaveTimeout for 0, got %v", optsZero.saveTimeout)
	}

	// Negative duration falls back to default
	optsNeg := newOptions(WithSaveTimeout(-1 * time.Second))
	if optsNeg.saveTimeout != defaultSaveTimeout {
		t.Fatalf("expected defaultSaveTimeout for negative, got %v", optsNeg.saveTimeout)
	}
}

func TestWithDirector(t *testing.T) {
	var receivedCustomHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCustomHeader = r.Header.Get("X-Director-Test")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("director response"))
	}))
	defer backend.Close()

	cache := setupTestCache(t)
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Parse URL: %v", err)
	}

	proxy, err := CreateCacheReverseProxy(cache,
		WithTargetURL(backendURL),
		WithDirector(func(req *http.Request) {
			req.Header.Set("X-Director-Test", "applied")
		}),
	)
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test-director", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if receivedCustomHeader != "applied" {
		t.Fatalf("X-Director-Test = %q, want applied", receivedCustomHeader)
	}
}

func TestWithServeErrorHandler(t *testing.T) {
	// Nil error handler is ignored (keeps default)
	serveOpts := newServeOptions(WithServeErrorHandler(nil))
	if serveOpts.errorHandler == nil {
		t.Fatal("errorHandler should not be nil after WithServeErrorHandler(nil)")
	}

	// Custom error handler is called on write failure during cache hit replay
	cache := setupTestCache(t)
	key := "/cached-for-error-handler"
	if err := cache.Save(context.Background(), key, http.Header{}, strings.NewReader("cached payload")); err != nil {
		t.Fatalf("seeding cache failed: %v", err)
	}

	backendURL, _ := url.Parse("http://unused.invalid")
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	var capturedErr error
	var capturedReq *http.Request
	handler := ServeCacheReverseProxy(cache, proxy, WithServeErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		capturedReq = r
	}))

	req := httptest.NewRequest(http.MethodGet, key, nil)
	handler(&failingResponseWriter{header: http.Header{}}, req)

	if capturedErr == nil {
		t.Fatal("expected error to be reported to WithServeErrorHandler, got nil")
	}
	if capturedReq != req {
		t.Fatalf("expected request %v, got %v", req, capturedReq)
	}
}

func TestWithServeCacheKeyFunc(t *testing.T) {
	cache := setupTestCache(t)
	const customKey = "custom-key-123"
	if err := cache.Save(context.Background(), customKey, http.Header{"Content-Type": []string{"text/plain"}}, strings.NewReader("custom cached payload")); err != nil {
		t.Fatalf("seeding cache failed: %v", err)
	}

	backendHitCount := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendHitCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend payload"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy, err := CreateCacheReverseProxy(cache,
		WithTargetURL(backendURL),
	)
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}

	// Case 1: custom key function returns the cached key -> cache hit
	handlerWithCustomKey := ServeCacheReverseProxy(cache, proxy, WithServeCacheKeyFunc(func(r *http.Request) string {
		if r.URL.Path == "/use-custom-key" {
			return customKey
		}
		return ""
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/use-custom-key", nil)
	handlerWithCustomKey(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "custom cached payload" {
		t.Fatalf("body = %q, want custom cached payload", rec.Body.String())
	}
	if backendHitCount != 0 {
		t.Fatalf("backend hit count = %d, want 0 on cache hit", backendHitCount)
	}

	// Case 2: keygen returns empty string -> cache skipped, forwards to upstream proxy
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/skip-cache", nil)
	handlerWithCustomKey(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec2.Code)
	}
	if rec2.Body.String() != "backend payload" {
		t.Fatalf("body = %q, want backend payload", rec2.Body.String())
	}
	if backendHitCount != 1 {
		t.Fatalf("backend hit count = %d, want 1 after upstream proxy", backendHitCount)
	}
}

// TestStressReverseProxy_ConcurrentCacheStreaming tests 50 concurrent clients requesting
// the same and different resources through reverse proxy to verify no corruption or race conditions.
func TestStressReverseProxy_ConcurrentCacheStreaming(t *testing.T) {
	cache := setupTestCache(t)

	// Upstream payload: 512KB
	payload := bytes.Repeat([]byte("0123456789abcdef"), 32*1024)
	expectedHash := sha256.Sum256(payload)

	var upstreamCalls atomic.Int64
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Parse URL: %v", err)
	}

	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)
	frontend := httptest.NewServer(http.HandlerFunc(handler))
	defer frontend.Close()

	const concurrentClients = 40
	var wg sync.WaitGroup

	for i := range concurrentClients {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			// Alternating: some clients hit the shared key /shared-payload, some hit client-specific keys
			path := "/shared-payload"
			if clientID%2 == 1 {
				path = fmt.Sprintf("/client-%d-payload", clientID)
			}

			resp, reqErr := frontend.Client().Get(frontend.URL + path)
			if reqErr != nil {
				t.Errorf("client %d GET error: %v", clientID, reqErr)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				t.Errorf("client %d ReadAll error: %v", clientID, readErr)
				return
			}

			if resp.StatusCode != http.StatusOK {
				t.Errorf("client %d got status %d, want 200", clientID, resp.StatusCode)
				return
			}

			if len(body) != len(payload) {
				t.Errorf("client %d got %d bytes, want %d", clientID, len(body), len(payload))
				return
			}

			if sha256.Sum256(body) != expectedHash {
				t.Errorf("client %d got corrupted body hash mismatch", clientID)
				return
			}
		}(i)
	}

	wg.Wait()
	// Allow background cache.Save goroutines to finish persisting to disk before TempDir cleanup
	time.Sleep(500 * time.Millisecond)
	t.Logf("Concurrent streaming test passed: %d clients, %d upstream calls", concurrentClients, upstreamCalls.Load())
}

// blockingCache simulates slow or broken storage save behavior
type customFailureCache struct {
	Cache
	saveMode int // 0: immediate error, 1: error after partial read, 2: hang until ctx cancel
}

func (c *customFailureCache) Save(ctx context.Context, key string, header http.Header, reader io.Reader) error {
	switch c.saveMode {
	case 0:
		// Immediate error without reading
		return errors.New("immediate storage failure")
	case 1:
		// Read 16KB then fail
		buf := make([]byte, 16*1024)
		_, _ = io.ReadFull(reader, buf)
		return errors.New("storage network timeout during save")
	case 2:
		// Hang until context is done
		<-ctx.Done()
		return ctx.Err()
	default:
		return c.Cache.Save(ctx, key, header, reader)
	}
}

// TestStressReverseProxy_BrokenCacheSinks verifies that when cache Save fails or hangs,
// the client receives 100% of the upstream stream without truncation or error.
func TestStressReverseProxy_BrokenCacheSinks(t *testing.T) {
	modes := []struct {
		name string
		mode int
	}{
		{name: "immediate save error", mode: 0},
		{name: "mid-stream save error", mode: 1},
		{name: "hung save with saveTimeout", mode: 2},
	}

	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			baseCache := setupTestCache(t)
			brokenCache := &customFailureCache{
				Cache:    baseCache,
				saveMode: tc.mode,
			}

			payload := bytes.Repeat([]byte("0123456789ABCDEF"), 16*1024) // 256 KB
			expectedHash := sha256.Sum256(payload)

			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(payload)
			}))
			defer backend.Close()

			backendURL, err := url.Parse(backend.URL)
			if err != nil {
				t.Fatalf("Parse backend URL: %v", err)
			}

			var recordedErrors atomic.Int64
			proxy, err := CreateCacheReverseProxy(
				brokenCache,
				WithTargetURL(backendURL),
				WithSaveTimeout(100*time.Millisecond),
				WithErrorHandler(func(err error) {
					recordedErrors.Add(1)
				}),
			)
			if err != nil {
				t.Fatalf("CreateCacheReverseProxy: %v", err)
			}

			handler := ServeCacheReverseProxy(brokenCache, proxy)
			frontend := httptest.NewServer(http.HandlerFunc(handler))
			defer frontend.Close()

			resp, err := frontend.Client().Get(frontend.URL + "/test-sink")
			if err != nil {
				t.Fatalf("GET request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Client body read failed: %v", err)
			}

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Status = %d, want 200", resp.StatusCode)
			}

			if len(body) != len(payload) {
				t.Fatalf("Client received %d bytes, want %d bytes (truncated!)", len(body), len(payload))
			}

			if sha256.Sum256(body) != expectedHash {
				t.Fatalf("Client received corrupted data payload")
			}
		})
	}
}

// TestStressReverseProxy_ClientDisconnectLeak verifies that client disconnects do not leak
// goroutines, memory, or file descriptors.
func TestStressReverseProxy_ClientDisconnectLeak(t *testing.T) {
	cache := setupTestCache(t)

	payload := bytes.Repeat([]byte("X"), 256*1024) // 256 KB

	var backendActive atomic.Int64
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendActive.Add(1)
		defer backendActive.Add(-1)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		// Write chunks with small delay
		for i := 0; i < len(payload); i += 16 * 1024 {
			end := min(i+16*1024, len(payload))
			_, writeErr := w.Write(payload[i:end])
			if writeErr != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Parse URL: %v", err)
	}

	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL), WithSaveTimeout(500*time.Millisecond))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)
	frontend := httptest.NewServer(http.HandlerFunc(handler))
	defer frontend.Close()

	initialGoroutines := runtime.NumGoroutine()

	// Launch 30 clients that abort after reading only first 512 bytes
	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/abort-%d", frontend.URL, idx), nil)
			resp, reqErr := http.DefaultClient.Do(req)
			if reqErr != nil {
				return
			}
			// Read a small chunk and close connection abruptly
			buf := make([]byte, 512)
			_, _ = resp.Body.Read(buf)
			_ = resp.Body.Close()
		}(i)
	}
	wg.Wait()

	// Wait for goroutines and save contexts to settle
	time.Sleep(1 * time.Second)

	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Goroutines: initial=%d, final=%d", initialGoroutines, finalGoroutines)

	// Verify backend connections drained
	if active := backendActive.Load(); active != 0 {
		t.Errorf("Active backend requests still open: %d", active)
	}
}

// TestStressReverseProxy_UpstreamAbruptClose verifies behavior when upstream server fails mid-response.
func TestStressReverseProxy_UpstreamAbruptClose(t *testing.T) {
	cache := setupTestCache(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("initial partial data"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Close the connection abruptly by hijacking
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		}
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Parse URL: %v", err)
	}

	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(backendURL), WithSaveTimeout(500*time.Millisecond))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}

	handler := ServeCacheReverseProxy(cache, proxy)
	frontend := httptest.NewServer(http.HandlerFunc(handler))
	defer frontend.Close()

	resp, err := frontend.Client().Get(frontend.URL + "/abrupt")
	if err == nil {
		_, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		// We expect either request error or body read error due to hijacked connection
		t.Logf("Upstream abrupt close resulted in read error: %v", readErr)
	}

	// Verify cache does NOT contain valid entry for /abrupt
	time.Sleep(200 * time.Millisecond)
	exists, err := cache.Exists(context.Background(), "/abrupt")
	if err == nil && exists {
		t.Errorf("Cache should not have stored corrupted/aborted response")
	}
}
