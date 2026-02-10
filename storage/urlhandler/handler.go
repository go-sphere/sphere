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
	basePath      string
	publicURL     *url.URL
}

// NewHandler creates a new URL handler with the specified public base URL.
// The public base URL is used to generate full URLs for storage keys.
func NewHandler(public string) (*Handler, error) {
	base, err := url.Parse(public)
	if err != nil {
		return nil, err
	}
	baseStr := base.String()
	baseStr = strings.TrimSuffix(baseStr, "/")
	sanitizedPath := strings.Trim(base.EscapedPath(), "/")

	return &Handler{
		publicURLBase: baseStr,
		basePath:      sanitizedPath,
		publicURL:     base,
	}, nil
}

// GenerateURL creates a public URL for the given storage key.
// If the key already contains a full URL, it returns the key unchanged.
// The default handler ignores params.
func (n *Handler) GenerateURL(key string, params ...url.Values) string {
	return n.generateURL(key)
}

// GenerateURLs creates public URLs for multiple storage keys in batch.
// The default handler ignores params.
func (n *Handler) GenerateURLs(keys []string, params ...url.Values) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		urls[i] = n.generateURL(key)
	}
	return urls
}

func (n *Handler) generateURL(key string) string {
	if key == "" {
		return ""
	}
	if hasHttpScheme(key) {
		return key
	}
	result, err := url.JoinPath(n.publicURLBase, key)
	if err != nil {
		return ""
	}
	return result
}

// ExtractKeyFromURLWithMode extracts the storage key from a URL with optional host verification.
// When strict is true, it validates that the URL belongs to the configured host.
// Returns an error if strict mode validation fails.
func (n *Handler) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	if uri == "" {
		return "", nil
	}
	if !hasHttpScheme(uri) {
		return strings.TrimPrefix(uri, "/"), nil
	}

	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if strict {
		if !sameHost(u, n.publicURL) {
			return "", ErrorNotVerifyHost
		}
	}
	path := strings.TrimPrefix(u.EscapedPath(), "/")

	if basePath := n.basePath; basePath != "" {
		switch {
		case path == basePath: // request path matches base exactly, so the key is empty
			path = ""
		case strings.HasPrefix(path, basePath+"/"): // request path is under base path, remove the prefix
			path = path[len(basePath)+1:]
		case strict: // base path mismatch in strict mode, reject
			return "", ErrorNotVerifyHost
		}
	}

	key, err := url.PathUnescape(path)
	if err != nil {
		return "", err
	}
	return key, nil
}

// ExtractKeyFromURL extracts the storage key from a URL with strict host verification enabled.
// Returns an empty string if host verification fails or if there's a parsing error.
func (n *Handler) ExtractKeyFromURL(uri string) string {
	key, err := n.ExtractKeyFromURLWithMode(uri, true)
	if err != nil {
		return ""
	}
	return key
}

func sameHost(target, base *url.URL) bool {
	if target == nil || base == nil {
		return false
	}
	if !strings.EqualFold(target.Hostname(), base.Hostname()) {
		return false
	}

	userPort := target.Port()
	basePort := base.Port()

	switch {
	case basePort == "" && userPort == "":
		return true
	case basePort == userPort:
		return true
	case basePort == "":
		return userPort == defaultPortForScheme(base.Scheme)
	case userPort == "":
		return basePort == defaultPortForScheme(target.Scheme)
	default:
		return false
	}
}

func hasHttpScheme(uri string) bool {
	return hasPrefixFold(uri, "http://") || hasPrefixFold(uri, "https://")
}

func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

func defaultPortForScheme(s string) string {
	switch strings.ToLower(s) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}
