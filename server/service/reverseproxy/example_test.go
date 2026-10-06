package reverseproxy_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/server/service/reverseproxy"
	"github.com/go-sphere/sphere/storage/kvcache"
)

func ExampleServeCacheReverseProxy() {
	var upstreamHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "report for "+r.URL.Path)
	}))
	defer upstream.Close()

	// Headers and bodies both live in memory here; production setups usually
	// keep bodies in a storage driver such as storage/local.
	headers := memory.NewByteCache()
	defer func() { _ = headers.Close() }()
	bodyCache := memory.NewByteCache()
	defer func() { _ = bodyCache.Close() }()
	bodies, err := kvcache.NewClient(kvcache.Config{}, bodyCache)
	if err != nil {
		fmt.Println("storage:", err)
		return
	}
	cache := reverseproxy.NewByteCache(headers, bodies)

	target, err := url.Parse(upstream.URL)
	if err != nil {
		fmt.Println("target:", err)
		return
	}
	proxy, err := reverseproxy.CreateCacheReverseProxy(cache, reverseproxy.WithTargetURL(target))
	if err != nil {
		fmt.Println("proxy:", err)
		return
	}
	handler := reverseproxy.ServeCacheReverseProxy(cache, proxy)

	get := func() {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/reports/1", nil))
		fmt.Println(rec.Code, rec.Body.String())
	}

	get() // miss: fetched upstream and saved in the background

	// The save is asynchronous; wait (bounded) until the entry is stored.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if ok, _ := cache.Exists(ctx, "/reports/1"); ok {
			break
		}
		select {
		case <-ctx.Done():
			fmt.Println("entry was not cached")
			return
		case <-time.After(5 * time.Millisecond):
		}
	}

	get() // hit: replayed from the cache
	fmt.Println("upstream hits:", upstreamHits.Load())
	// Output:
	// 200 report for /reports/1
	// 200 report for /reports/1
	// upstream hits: 1
}
