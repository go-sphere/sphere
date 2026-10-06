// Package reverseproxy is a caching reverse proxy for net/http: eligible
// upstream responses are stored once and replayed to later clients without
// contacting the upstream.
//
// The entry points are a [Cache] (usually [NewByteCache], which keeps headers
// in a cache.ByteCache and bodies in a storage.Storage),
// [CreateCacheReverseProxy], which builds the upstream proxy that fills the
// cache, and [ServeCacheReverseProxy], which serves cache hits and otherwise
// forwards to that proxy.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/cache/memory"
//		"github.com/go-sphere/sphere/server/httpz"
//		"github.com/go-sphere/sphere/server/service/reverseproxy"
//		"github.com/go-sphere/sphere/storage/local"
//	)
//
//	headers := memory.NewByteCache()
//	defer headers.Close()
//	bodies, err := local.NewClient(local.Config{RootDir: "./data/proxy-cache"})
//	if err != nil {
//		return err
//	}
//	cache := reverseproxy.NewByteCache(headers, bodies)
//
//	target, err := url.Parse("https://upstream.example.com")
//	if err != nil {
//		return err
//	}
//	proxy, err := reverseproxy.CreateCacheReverseProxy(cache, reverseproxy.WithTargetURL(target))
//	if err != nil {
//		return err
//	}
//	handler := http.HandlerFunc(reverseproxy.ServeCacheReverseProxy(cache, proxy))
//
//	// Serve it with net/http, or mount it on an httpx engine:
//	err = httpz.MountStdAll(engine.Group(""), "/*filepath", handler, http.MethodGet)
//
// # Caching rules
//
//   - The default cache key is the request URI of GET requests; other methods
//     are never cached. The key does not include the Host, so multi-vhost
//     deployments need WithCacheKeyFunc and WithServeCacheKeyFunc.
//   - The default check stores only 200 responses to requests without
//     Authorization or Cookie, and refuses Set-Cookie, Content-Encoding other
//     than identity, no-store/private/no-cache/must-revalidate, a non-positive
//     max-age, and Vary on anything but Accept-Encoding.
//   - Entries live as long as the backing cache keeps them; the proxy never
//     revalidates with the upstream.
//   - Saving runs in the background on a context detached from the request
//     (30s by default). A failing or slow cache never affects the client
//     stream; Load errors other than a miss are logged and the request goes
//     upstream.
package reverseproxy
