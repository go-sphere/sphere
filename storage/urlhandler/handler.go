package urlhandler

import (
	"fmt"
	"net/url"
	"strings"
)

// ErrorNotVerifyHost is returned when URL host verification fails in strict mode.
var ErrorNotVerifyHost = fmt.Errorf("not verify host")

// Handler provides URL generation and key extraction for storage backends.
// It manages the relationship between storage keys and their public URLs.
type Handler struct {
	publicURLBase string
	publicURLHost string
}

// NewHandler creates a new URL handler with the specified public base URL.
// The public base URL is used to generate full URLs for storage keys.
func NewHandler(public string) (*Handler, error) {
	base, err := url.Parse(public)
	if err != nil {
		return nil, err
	}
	return &Handler{
		publicURLBase: base.String(),
		publicURLHost: base.Host,
	}, nil
}

// hasHttpScheme checks if the given URI has an HTTP or HTTPS scheme.
func (n *Handler) hasHttpScheme(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}

// GenerateURL creates a public URL for the given storage key.
// If the key already contains a full URL, it returns the key unchanged.
func (n *Handler) GenerateURL(key string) string {
	if key == "" {
		return ""
	}
	if n.hasHttpScheme(key) {
		return key
	}
	result, err := url.JoinPath(n.publicURLBase, key)
	if err != nil {
		return ""
	}
	return result
}

// GenerateURLs creates public URLs for multiple storage keys in batch.
func (n *Handler) GenerateURLs(keys []string) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		urls[i] = n.GenerateURL(key)
	}
	return urls
}

// ExtractKeyFromURLWithMode extracts the storage key from a URL with optional host verification.
// When strict is true, it validates that the URL belongs to the configured host.
// Returns an error if strict mode validation fails.
func (n *Handler) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	if uri == "" {
		return "", nil
	}
	// 不是 http 或者 https 开头的直接返回
	if !n.hasHttpScheme(uri) {
		return strings.TrimPrefix(uri, "/"), nil
	}
	// 解析URL
	u, err := url.Parse(uri)
	if err != nil {
		return "", nil
	}
	if strict {
		// 不是以CDN域名开头的直接返回或者报错
		if u.Host != n.publicURLHost {
			return "", ErrorNotVerifyHost
		}
	}
	u.RawQuery = ""
	u.RawFragment = ""
	// 返回去掉base url的key
	return strings.TrimPrefix(strings.TrimPrefix(u.String(), n.publicURLBase), "/"), nil
}

// ExtractKeyFromURL extracts the storage key from a URL with strict host verification enabled.
// Returns an empty string if the URL format is invalid or host verification fails.
func (n *Handler) ExtractKeyFromURL(uri string) string {
	key, _ := n.ExtractKeyFromURLWithMode(uri, true)
	return key
}
