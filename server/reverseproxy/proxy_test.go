package reverseproxy

import (
	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/storage/local"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestServeCacheReverseProxy(t *testing.T) {
	store, err := local.NewClient(&local.Config{
		RootDir:    "./temp",
		PublicBase: "/",
	})
	if err != nil {
		t.Fatal(err)
	}
	cache := NewByteCache(
		5*time.Minute,
		memory.NewByteCache(),
		store,
	)
	root, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	proxy, err := CreateCacheReverseProxy(cache, WithTargetURL(root))
	if err != nil {
		t.Fatal(err)
	}
	server := ServeCacheReverseProxy(NewProxyConfig().keygen, cache, proxy)
	_ = http.ListenAndServe(":9999", http.HandlerFunc(server))
}
