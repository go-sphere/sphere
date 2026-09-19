package reverseproxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/httpz"
)

// TestServeCacheReverseProxy mounts the cached reverse proxy on an httpx engine
// via httpz.MountStdAll: the first request goes upstream, the second replays
// the cache with equal status, body, and upstream headers.
func TestServeCacheReverseProxy(t *testing.T) {
	var upstreamHits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "payload:"+r.URL.Path)
	}))
	defer upstream.Close()

	cache := setupTestCache(t)
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(target))
	if err != nil {
		t.Fatalf("CreateCacheReverseProxy: %v", err)
	}
	handler := http.HandlerFunc(ServeCacheReverseProxy(cache, proxy))

	engine := stdx.New()
	if err := httpz.MountStdAll(engine.Group(""), "/proxy/*filepath", handler, http.MethodGet); err != nil {
		t.Fatalf("MountStdAll: %v", err)
	}
	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		t.Fatal("the engine does not support in-process dispatch")
	}

	const reqPath = "/proxy/assets/app.js"
	do := func() (int, http.Header, string) {
		t.Helper()
		resp, err := tr.Do(httptest.NewRequest(http.MethodGet, "http://example.com"+reqPath, nil))
		if err != nil {
			t.Fatalf("GET %s: %v", reqPath, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode, resp.Header, string(body)
	}

	firstStatus, firstHeader, firstBody := do()
	if firstStatus != http.StatusOK || firstBody != "payload:"+reqPath {
		t.Fatalf("first response = %d %q", firstStatus, firstBody)
	}
	if hits := upstreamHits.Load(); hits != 1 {
		t.Fatalf("upstream hits after first request = %d, want 1", hits)
	}

	// The cache save runs detached from the request; wait for the
	// entry before asserting the hit path.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if ok, err := cache.Exists(context.Background(), reqPath); err == nil && ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cache entry did not appear within deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}

	secondStatus, secondHeader, secondBody := do()
	if hits := upstreamHits.Load(); hits != 1 {
		t.Fatalf("upstream hits after cache hit = %d, want 1", hits)
	}
	if secondStatus != firstStatus || secondBody != firstBody {
		t.Fatalf("cache replay = %d %q, want %d %q", secondStatus, secondBody, firstStatus, firstBody)
	}
	if firstHeader.Get("X-Upstream") != "yes" || secondHeader.Get("X-Upstream") != "yes" {
		t.Fatalf("X-Upstream header lost: first=%q second=%q",
			firstHeader.Get("X-Upstream"), secondHeader.Get("X-Upstream"))
	}
}
