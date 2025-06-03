package reverseproxy

import (
	"context"
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

type ProxyConfig struct {
	target   *url.URL
	director func(*http.Request)
	keygen   RequestCacheKeyFunc
	checker  ResponseCacheCheckFunc
}

func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{
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
	}
}

type ConfigOption = func(*ProxyConfig)

func WithTargetURL(target *url.URL) ConfigOption {
	return func(config *ProxyConfig) {
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

func WithDirector(director func(*http.Request)) ConfigOption {
	return func(config *ProxyConfig) {
		config.director = director
	}
}

func WithCacheKeyFunc(cacheKeyFunc RequestCacheKeyFunc) ConfigOption {
	return func(config *ProxyConfig) {
		config.keygen = cacheKeyFunc
	}
}

func WithResponseCacheCheck(checker ResponseCacheCheckFunc) ConfigOption {
	return func(config *ProxyConfig) {
		config.checker = checker
	}
}

func ignoreCloseError(closer func() error) {
	_ = closer()
}

func CreateCacheReverseProxy(cache Cache, options ...ConfigOption) (*httputil.ReverseProxy, error) {
	conf := NewProxyConfig()
	for _, option := range options {
		option(conf)
	}
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
		go func() {
			defer ignoreCloseError(originalBody.Close)
			defer ignoreCloseError(clientPipeWriter.Close)
			defer ignoreCloseError(cachePipeWriter.Close)
			multiWriter := io.MultiWriter(clientPipeWriter, cachePipeWriter)
			_, _ = io.Copy(multiWriter, originalBody)
		}()
		go func() {
			defer cacheFlags.Delete(key)
			defer ignoreCloseError(cachePipeReader.Close)
			_ = cache.Save(context.Background(), key, resp.Header, cachePipeReader)
		}()

		return nil
	}
	return proxy, nil
}

func ServeCacheReverseProxy(keygen RequestCacheKeyFunc, cache Cache, proxy *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		key := keygen(r)
		if key != "" {
			if header, body, err := cache.Load(r.Context(), key); err == nil {
				for k, v := range header {
					for _, vv := range v {
						w.Header().Add(k, vv)
					}
				}
				w.WriteHeader(http.StatusOK)
				if closer, ok := body.(io.Closer); ok {
					defer ignoreCloseError(closer.Close)
				}
				_, _ = io.Copy(w, body)
				return
			}
		}
		proxy.ServeHTTP(w, r)
	}
}
