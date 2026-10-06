package urlhandler

import (
	"fmt"
	"net/url"
	"strings"
)

// ErrHostVerificationFailed is returned in strict mode when a URL's host
// (with default ports for http/https considered equal) or path prefix does not
// match the public base.
var ErrHostVerificationFailed = fmt.Errorf("not verify host")

// ErrorNotVerifyHost is an alias for ErrHostVerificationFailed for backwards compatibility.
var ErrorNotVerifyHost = ErrHostVerificationFailed

// ErrInvalidKeyPath is returned when an extracted key contains a ".." path
// segment. Every storage driver refuses such keys, so persisting one would
// diverge from the object it can never address.
var ErrInvalidKeyPath = fmt.Errorf("invalid key path")

// Handler provides URL generation and key extraction for storage backends.
// It manages the relationship between storage keys and their public URLs.
// Create it with NewHandler. A Handler is immutable after construction and
// safe for concurrent use; s3, qiniu, and fileserver embed or wrap it to
// implement storage.URLHandler.
type Handler struct {
	publicURLBase string
	basePath      string
	publicURL     *url.URL
}

// NewHandler creates a new URL handler with the specified public base URL,
// such as "https://cdn.example.com/assets". A trailing slash is ignored.
// It returns an error only when public cannot be parsed by url.Parse; it does
// not require a scheme or host.
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

// GenerateURL creates a public URL for the given storage key by joining it
// onto the public base. Keys that already look like http:// or https:// are
// returned unchanged. It returns "" for an empty key, for a key containing a
// ".." segment, and when joining fails. The handler ignores params.
func (n *Handler) GenerateURL(key string, params ...url.Values) string {
	return n.generateURL(key)
}

// GenerateURLs creates public URLs for multiple storage keys in batch, with
// the same rules as GenerateURL; the result has one entry per key in order.
// The handler ignores params.
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
	if containsDotDot(key) {
		// url.JoinPath would resolve ".." against the base and silently fold
		// the URL to a sibling of the public base. No storage driver accepts
		// such a key, so refuse to mint a URL for it instead of emitting one
		// that addresses the wrong origin/path.
		return ""
	}
	result, err := url.JoinPath(n.publicURLBase, key)
	if err != nil {
		return ""
	}
	return result
}

// ExtractKeyFromURLWithMode extracts the storage key from a URL.
// An empty uri yields "". A value without an http:// or https:// scheme is
// treated as a key: its leading "/" is dropped and it is returned as is.
// For a URL, the public base path is stripped and the remainder is
// path-unescaped. When strict is true, the URL host must match the public base
// and the path must be under that base; otherwise ErrHostVerificationFailed is
// returned. When strict is false, any host is accepted, the base path is
// stripped when the path starts with it, and otherwise the whole path becomes
// the key. A key with a ".." segment fails with
// ErrInvalidKeyPath.
func (n *Handler) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	if uri == "" {
		return "", nil
	}
	if !hasHttpScheme(uri) {
		key := strings.TrimPrefix(uri, "/")
		// Same refusal as the full-URL branch below: a ".."-carrying key can
		// never address an object, so persisting it only produces a dirty
		// value that later storage calls fail on.
		if containsDotDot(key) {
			return "", ErrInvalidKeyPath
		}
		return key, nil
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
	if containsDotDot(key) {
		// Drivers reject ".." segments (storage.NormalizeKey), so a key
		// extracted here could never address an object — persisting it only
		// produces a dirty value that later storage calls fail on.
		return "", ErrInvalidKeyPath
	}
	return key, nil
}

// containsDotDot reports whether a decoded key contains a ".." path segment
// (or its encoded alias, which PathUnescape has already folded to "..").
func containsDotDot(key string) bool {
	for segment := range strings.SplitSeq(key, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

// ExtractKeyFromURL extracts the storage key from a URL with strict host verification enabled.
// Returns an empty string if host verification fails or if there's a parsing error.
// Use ExtractKeyFromURLWithMode to distinguish those failures.
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
