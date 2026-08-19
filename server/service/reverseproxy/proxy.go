package reverseproxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sphere/sphere/core/safe"
	"github.com/go-sphere/sphere/log"
)

// cacheStatusHeader carries the original upstream status code inside the persisted
// header blob so the cached response can be replayed with its real status code.
// It is stripped from the response headers before they are sent to the client.
const cacheStatusHeader = "X-Sphere-Cache-Status"

type (
	RequestCacheKeyFunc    func(*http.Request) string
	ResponseCacheCheckFunc func(*http.Response) bool
)

// defaultSaveTimeout bounds how long persisting one response to the cache may
// take. The save runs on a context detached from the request, so nothing else
// would ever stop it.
const defaultSaveTimeout = 30 * time.Second

type Options struct {
	target       *url.URL
	director     func(*http.Request)
	errorHandler func(error)
	keygen       RequestCacheKeyFunc
	checker      ResponseCacheCheckFunc
	saveTimeout  time.Duration
}

type Option = func(*Options)

func defaultCacheKey(request *http.Request) string {
	if request.Method != http.MethodGet {
		return ""
	}
	return request.URL.RequestURI()
}

// defaultResponseCacheCheck decides whether a response may enter a cache that is
// shared by every client. The cache key is derived from the request URI alone,
// so anything stored here is replayable by an anonymous caller — which makes
// "is this response private?" the only question that matters.
//
// The rules follow RFC 9111: a request carrying credentials is assumed to
// produce a user-specific response, and an upstream that says no-store, private,
// or sets a cookie is taken at its word. Vary is honoured by refusing to store,
// not by negotiating: the cache key is computed from the request alone on
// lookup, where the response's Vary is not yet known, so a varying response
// cannot be keyed correctly and must not be stored at all.
//
// Callers that know better can replace this via WithResponseCacheCheck — for
// example to cache per-user responses under a key that includes the user.
func defaultResponseCacheCheck(resp *http.Response) bool {
	if resp.StatusCode != http.StatusOK {
		return false
	}
	if resp.Request == nil || resp.Request.Method != http.MethodGet {
		return false
	}
	// A credentialed request yields a response scoped to that caller.
	if resp.Request.Header.Get("Authorization") != "" || resp.Request.Header.Get("Cookie") != "" {
		return false
	}
	// Storing a Set-Cookie would hand one client's session to the next.
	if len(resp.Header.Values("Set-Cookie")) > 0 {
		return false
	}
	if hasNoStoreDirective(resp.Header.Values("Cache-Control")) {
		return false
	}
	return varyIsCacheable(resp.Header.Values("Vary"))
}

// hasNoStoreDirective reports whether Cache-Control forbids shared storage.
// "private" counts: this cache is shared by definition.
func hasNoStoreDirective(values []string) bool {
	for _, value := range values {
		for _, directive := range strings.Split(value, ",") {
			switch strings.ToLower(strings.TrimSpace(directive)) {
			case "no-store", "private":
				return true
			}
		}
	}
	return false
}

// varyIsCacheable reports whether a response's Vary can be satisfied by a cache
// keyed on the request URI alone. Only Accept-Encoding qualifies, because the
// default director strips that header before the request reaches the upstream,
// leaving a single variant. Everything else — including "*" — means the stored
// entry could be served to a request that should have received a different one.
func varyIsCacheable(values []string) bool {
	for _, value := range values {
		for _, field := range strings.Split(value, ",") {
			if field = strings.TrimSpace(field); field == "" {
				continue
			}
			if !strings.EqualFold(field, "Accept-Encoding") {
				return false
			}
		}
	}
	return true
}

func newOptions(opts ...Option) *Options {
	conf := &Options{
		keygen:  defaultCacheKey,
		checker: defaultResponseCacheCheck,
		errorHandler: func(err error) {
			// default: do nothing
		},
		saveTimeout: defaultSaveTimeout,
	}
	for _, opt := range opts {
		opt(conf)
	}
	return conf
}

// WithSaveTimeout bounds how long persisting a single response to the cache may
// take. A non-positive duration restores the default.
func WithSaveTimeout(timeout time.Duration) Option {
	return func(config *Options) {
		if timeout <= 0 {
			timeout = defaultSaveTimeout
		}
		config.saveTimeout = timeout
	}
}

func WithTargetURL(target *url.URL) Option {
	return func(config *Options) {
		config.target = target
		if config.director == nil {
			config.director = func(request *http.Request) {
				request.URL.Scheme = target.Scheme
				request.URL.Host = target.Host
				request.Host = target.Host
				request.Header.Del("Origin")          // remove origin header
				request.Header.Del("Referer")         // remove referer header
				request.Header.Del("Accept-Encoding") // remove gzip encoding
				//request.Header.Set("Origin", target.Scheme+"://"+target.Host)
				//request.Header.Set("Referer", target.Scheme+"://"+target.Host)
				//request.URL.Path = target.Path + request.URL.Path
				//if request.URL.RawQuery != "" {
				//	request.URL.Path += "?" + request.URL.RawQuery
				//}
			}
		}
	}
}

func WithDirector(director func(*http.Request)) Option {
	return func(config *Options) {
		config.director = director
	}
}

// WithErrorHandler sets the callback invoked when persisting a response to the
// cache fails. A nil handler is ignored, leaving the no-op default in place;
// storing it would turn every cache-save failure into a nil call in a background
// goroutine, where the panic is unrecoverable and takes the process down.
func WithErrorHandler(handler func(error)) Option {
	return func(config *Options) {
		if handler == nil {
			return
		}
		config.errorHandler = handler
	}
}

func WithCacheKeyFunc(cacheKeyFunc RequestCacheKeyFunc) Option {
	return func(config *Options) {
		config.keygen = cacheKeyFunc
	}
}

func WithResponseCacheCheck(checker ResponseCacheCheckFunc) Option {
	return func(config *Options) {
		config.checker = checker
	}
}

func ignoreCloseError(closer func() error) {
	_ = closer()
}

func CreateCacheReverseProxy(cache Cache, opts ...Option) (*httputil.ReverseProxy, error) {
	conf := newOptions(opts...)
	proxy := httputil.NewSingleHostReverseProxy(conf.target)
	if conf.director != nil {
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			conf.director(req)
		}
	}

	cacheFlags := sync.Map{}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if !conf.checker(resp) {
			return nil
		}
		key := conf.keygen(resp.Request)
		if key == "" {
			return nil // no cache key, do not cache
		}
		if _, ok := cacheFlags.Load(key); ok {
			return nil // already cached, do not cache again
		}
		cacheFlags.Store(key, struct{}{})
		clientPipeReader, clientPipeWriter := io.Pipe()
		cachePipeReader, cachePipeWriter := io.Pipe()

		originalBody := resp.Body
		resp.Body = clientPipeReader

		// goroutine to copy response body to both client and cache
		safe.Go(func() {
			defer func() {
				ignoreCloseError(originalBody.Close)
				ignoreCloseError(clientPipeWriter.Close)
				ignoreCloseError(cachePipeWriter.Close)
			}()
			// The two sinks are written separately rather than through an
			// io.MultiWriter. A MultiWriter fails as a whole as soon as either
			// side errors, which meant a failing cache write aborted the copy to
			// the client: the client kept its 200 and a truncated body, directly
			// contradicting the promise below that a cache failure does not
			// affect it. A cache backend that rejects the write outright — a
			// full disk, an expired credential, an invalid key — reads nothing
			// at all, so this was reachable on the very first chunk.
			buf := make([]byte, 32*1024)
			cacheBroken := false
			for {
				n, readErr := originalBody.Read(buf)
				if n > 0 {
					if _, err := clientPipeWriter.Write(buf[:n]); err != nil {
						// The client is gone; the cache entry would be partial.
						_ = cachePipeWriter.CloseWithError(err)
						return
					}
					if !cacheBroken {
						if _, err := cachePipeWriter.Write(buf[:n]); err != nil {
							// Give up on caching, keep serving the client. The
							// reader side reports the failure through Save.
							cacheBroken = true
							_ = cachePipeWriter.CloseWithError(err)
						}
					}
				}
				if readErr != nil {
					if readErr != io.EOF {
						_ = clientPipeWriter.CloseWithError(readErr)
						if !cacheBroken {
							_ = cachePipeWriter.CloseWithError(readErr)
						}
					}
					return
				}
			}
		})

		// goroutine to save cache
		safe.Go(func() {
			defer func() {
				cacheFlags.Delete(key)
				ignoreCloseError(cachePipeReader.Close)
			}()
			// Detached from the request, but bounded. Detaching alone let a
			// backend that never answers hold this goroutine, the copy
			// goroutine, the upstream body and the key's cacheFlags entry for
			// the life of the process — and because the flag is only cleared
			// here, that key was never cached again either.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(resp.Request.Context()), conf.saveTimeout)
			defer cancel()
			// Persist the real status code inside the header blob so it can be
			// replayed on cache hit. Clone to avoid leaking the internal header
			// to the client response.
			cacheHeader := resp.Header.Clone()
			// Never persist a Set-Cookie: a cached entry is replayed to every
			// later caller, so storing one would hand out the session it
			// belongs to. The default checker already refuses such responses;
			// this also covers a custom checker that does not.
			cacheHeader.Del("Set-Cookie")
			cacheHeader.Set(cacheStatusHeader, strconv.Itoa(resp.StatusCode))
			if err := cache.Save(ctx, key, cacheHeader, cachePipeReader); err != nil {
				// Cache save failed, but continue serving client
				// Error is silently ignored as cache is not critical
				conf.errorHandler(err)
			}
		})

		return nil
	}
	return proxy, nil
}

type ServeOptions struct {
	keygen       RequestCacheKeyFunc
	errorHandler func(http.ResponseWriter, *http.Request, error)
}

type ServeOption = func(*ServeOptions)

func newServeOptions(opts ...ServeOption) *ServeOptions {
	conf := &ServeOptions{
		keygen: defaultCacheKey,
		// A default is required, not merely convenient: the handler is called
		// whenever copying a cached body to the client fails, which happens on
		// every client that disconnects mid-download. Leaving it nil made that
		// ordinary event a nil call, and this option is entirely optional, so
		// nothing pointed the caller at it.
		errorHandler: func(http.ResponseWriter, *http.Request, error) {
			// default: do nothing
		},
	}
	for _, opt := range opts {
		opt(conf)
	}
	return conf
}

// WithServeErrorHandler sets the callback invoked when a cached response cannot
// be written to the client. A nil handler is ignored, leaving the no-op default
// in place.
func WithServeErrorHandler(handler func(http.ResponseWriter, *http.Request, error)) ServeOption {
	return func(opts *ServeOptions) {
		if handler == nil {
			return
		}
		opts.errorHandler = handler
	}
}

func WithServeCacheKeyFunc(keygen RequestCacheKeyFunc) ServeOption {
	return func(opts *ServeOptions) {
		opts.keygen = keygen
	}
}

func ServeCacheReverseProxy(cache Cache, proxy *httputil.ReverseProxy, opts ...ServeOption) func(http.ResponseWriter, *http.Request) {
	conf := newServeOptions(opts...)
	return func(w http.ResponseWriter, r *http.Request) {
		key := conf.keygen(r)
		if key != "" {
			header, body, err := cache.Load(r.Context(), key)
			switch {
			case err != nil:
				// A cache miss is expected; anything else is a cache-layer
				// failure that should not be silently swallowed.
				if !errors.Is(err, ErrCacheNotFound) {
					log.Error("reverseproxy: failed to load cache", log.String("key", key), log.Err(err))
				}
			case body != nil:
				// Replay the persisted status code instead of hardcoding 200,
				// then strip the internal header from the client response.
				status := http.StatusOK
				if s := header.Get(cacheStatusHeader); s != "" {
					// Validate the range before replaying: net/http panics on a
					// status outside 100-999, so a corrupted or poisoned cache
					// entry would otherwise take down the request goroutine.
					// Fall back to 200 for anything unparseable or out of range.
					if code, pErr := strconv.Atoi(s); pErr == nil && code >= 100 && code <= 599 {
						status = code
					}
					header.Del(cacheStatusHeader)
				}
				// Copy headers to response
				for k, v := range header {
					for _, vv := range v {
						w.Header().Add(k, vv)
					}
				}
				w.WriteHeader(status)
				defer ignoreCloseError(body.Close)
				if _, cErr := io.Copy(w, body); cErr != nil {
					conf.errorHandler(w, r, cErr)
					return
				}
				return
			}
		}
		proxy.ServeHTTP(w, r)
	}
}
