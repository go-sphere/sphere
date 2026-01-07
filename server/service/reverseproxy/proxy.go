package reverseproxy

import (
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

type (
	RequestCacheKeyFunc    func(*http.Request) string
	ResponseCacheCheckFunc func(*http.Response) bool
)

type Options struct {
	target       *url.URL
	director     func(*http.Request)
	errorHandler func(error)
	keygen       RequestCacheKeyFunc
	checker      ResponseCacheCheckFunc
}

type Option = func(*Options)

func newOptions(opts ...Option) *Options {
	conf := &Options{
		keygen: func(request *http.Request) string {
			if request.Method != http.MethodGet {
				return ""
			}
			return request.URL.Path
		},
		checker: func(resp *http.Response) bool {
			if resp.StatusCode != http.StatusOK {
				return false
			}
			if resp.Request.Method != http.MethodGet {
				return false
			}
			return true
		},
		errorHandler: func(err error) {
			// default: do nothing
		},
	}
	for _, opt := range opts {
		opt(conf)
	}
	return conf
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

func WithErrorHandler(handler func(error)) Option {
	return func(config *Options) {
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
		go func() {
			defer func() {
				ignoreCloseError(originalBody.Close)
				ignoreCloseError(clientPipeWriter.Close)
				ignoreCloseError(cachePipeWriter.Close)
			}()
			multiWriter := io.MultiWriter(clientPipeWriter, cachePipeWriter)
			if _, err := io.Copy(multiWriter, originalBody); err != nil {
				// Close with error to notify readers
				_ = clientPipeWriter.CloseWithError(err)
				_ = cachePipeWriter.CloseWithError(err)
			}
		}()

		// goroutine to save cache
		go func() {
			defer func() {
				cacheFlags.Delete(key)
				ignoreCloseError(cachePipeReader.Close)
			}()
			ctx := resp.Request.Context()
			if err := cache.Save(ctx, key, resp.Header, cachePipeReader); err != nil {
				// Cache save failed, but continue serving client
				// Error is silently ignored as cache is not critical
				conf.errorHandler(err)
			}
		}()

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
		keygen: func(request *http.Request) string {
			return request.URL.Path
		},
	}
	for _, opt := range opts {
		opt(conf)
	}
	return conf
}

func WithServeErrorHandler(handler func(http.ResponseWriter, *http.Request, error)) ServeOption {
	return func(opts *ServeOptions) {
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
			if header, body, err := cache.Load(r.Context(), key); err == nil && body != nil {
				// Copy headers to response
				for k, v := range header {
					for _, vv := range v {
						w.Header().Add(k, vv)
					}
				}
				w.WriteHeader(http.StatusOK)
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
