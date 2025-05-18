package reverseproxy

import (
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/storage/local"
)

func TestServeCacheReverseProxy(t *testing.T) {
	if os.Getenv("TEST_REVERSE_PROXY") != "true" {
		t.Skip()
	}
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
