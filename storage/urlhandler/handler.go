package urlhandler

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// ErrHostVerificationFailed is returned in strict mode when a URL's host
// (with default ports for http/https considered equal) or path prefix does not
// match the public base.
var ErrHostVerificationFailed = fmt.Errorf("not verify host")

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
// It returns an error when public cannot be parsed, or is not an absolute
// http or https URL with a host: keys only round-trip through
// ExtractKeyFromURL against such a base.
func NewHandler(public string) (*Handler, error) {
	base, err := url.Parse(public)
	if err != nil {
		return nil, err
	}
	if (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("urlhandler: public base %q must be an absolute http or https URL", public)
	}
	if base.RawQuery != "" || base.ForceQuery || base.Fragment != "" {
		return nil, fmt.Errorf("urlhandler: public base %q must not contain query or fragment", public)
	}
	if base.Path != "" {
		cleanPath := path.Clean("/" + strings.TrimPrefix(base.Path, "/"))
		if cleanPath == "/" {
			base.Path = ""
			base.RawPath = ""
		} else if cleanPath != strings.TrimRight(base.Path, "/") {
			base.Path = cleanPath
			base.RawPath = ""
		}
	}
	baseStr := base.String()
	baseStr = strings.TrimRight(baseStr, "/")
	sanitizedPath := strings.Trim(base.EscapedPath(), "/")

	return &Handler{
		publicURLBase: baseStr,
		basePath:      sanitizedPath,
		publicURL:     base,
	}, nil
}

// GenerateURL creates a public URL for the given storage key by joining it
// onto the public base, path-escaping each "/"-separated segment so the
// result round-trips through ExtractKeyFromURL. Keys that already look like
// http:// or https:// are
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
	if hasHttpScheme(key) {
		return key
	}
	if n == nil {
		return ""
	}
	key = strings.TrimLeft(key, "/")
	if key == "" || strings.ContainsRune(key, '\x00') || containsRelativeSegment(key) {
		// No storage driver accepts ".", "..", or NUL-carrying keys, so refuse
		// to mint a URL for them instead of emitting one that addresses the
		// wrong origin/path.
		return ""
	}
	// Escape each segment directly so a literal "%" or "?"/"#" in a key
	// round-trips through ExtractKeyFromURL instead of being decoded or
	// interpreted as query/fragment delimiters.
	var sb strings.Builder
	sb.Grow(len(n.publicURLBase) + 1 + len(key))
	sb.WriteString(n.publicURLBase)
	wroteSegment := false
	for segment := range strings.SplitSeq(key, "/") {
		if segment == "" {
			continue
		}
		sb.WriteByte('/')
		sb.WriteString(url.PathEscape(segment))
		wroteSegment = true
	}
	if !wroteSegment {
		return ""
	}
	return sb.String()
}

// ExtractKeyFromURLWithMode extracts the storage key from a URL.
// An empty uri yields "". A value without a URL scheme and without a
// protocol-relative "//host" prefix is treated as a key: its leading "/" is
// dropped and it is returned as is. Protocol-relative URLs ("//host/...") and
// non-HTTP(S) schemes (such as "ftp://..." or "file:///...") are rejected.
// For an HTTP(S) URL, a non-empty host is required; the public base path is
// stripped and the remainder is path-unescaped. When strict is true, the URL
// host must match the public base and the path must be under that base;
// otherwise ErrHostVerificationFailed is returned. When strict is false, any
// non-empty host is accepted, the base path is stripped when the path starts
// with it, and otherwise the whole path becomes the key. A key with a ".",
// "..", or NUL byte fails with ErrInvalidKeyPath.
func (n *Handler) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	if uri == "" {
		return "", nil
	}
	if strings.HasPrefix(uri, "//") && strings.TrimLeft(uri, "/") != "" {
		return "", fmt.Errorf("urlhandler: protocol-relative URL %q is not supported", uri)
	}
	if !hasHttpScheme(uri) {
		if hasSchemePrefix(uri) {
			return "", fmt.Errorf("urlhandler: unsupported URL scheme in %q", uri)
		}
		key := strings.TrimLeft(uri, "/")
		// Same refusal as the full-URL branch below: a ".", "..", or NUL-carrying
		// key can never address an object, so persisting it only produces a
		// dirty value that later storage calls fail on.
		if strings.ContainsRune(key, '\x00') || containsRelativeSegment(key) {
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
			return "", ErrHostVerificationFailed
		}
	} else if u.Host == "" {
		return "", fmt.Errorf("urlhandler: URL %q has empty host", uri)
	}
	escapedPath := strings.TrimLeft(u.EscapedPath(), "/")

	if basePath := n.basePath; basePath != "" {
		switch {
		case escapedPath == basePath: // request path matches base exactly, so the key is empty
			escapedPath = ""
		case strings.HasPrefix(escapedPath, basePath+"/"): // request path is under base path, remove the prefix
			escapedPath = strings.TrimLeft(escapedPath[len(basePath)+1:], "/")
		case strict: // base path mismatch in strict mode, reject
			return "", ErrHostVerificationFailed
		}
	}

	key, err := url.PathUnescape(escapedPath)
	if err != nil {
		return "", err
	}
	key = strings.TrimLeft(key, "/")
	if strings.ContainsRune(key, '\x00') || containsRelativeSegment(key) {
		// Drivers reject ".", "..", and NUL (storage.NormalizeKey), so a key
		// extracted here could never address an object.
		return "", ErrInvalidKeyPath
	}
	return key, nil
}

// containsRelativeSegment reports whether a decoded key contains a "." or ".."
// path segment (or its encoded alias, which PathUnescape has already folded).
func containsRelativeSegment(key string) bool {
	for segment := range strings.SplitSeq(key, "/") {
		if segment == "." || segment == ".." {
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
	if userPort == defaultPortForScheme(target.Scheme) {
		userPort = ""
	}
	basePort := base.Port()
	if basePort == defaultPortForScheme(base.Scheme) {
		basePort = ""
	}
	return userPort == basePort
}

func hasHttpScheme(uri string) bool {
	return hasPrefixFold(uri, "http://") || hasPrefixFold(uri, "https://")
}

func hasSchemePrefix(uri string) bool {
	scheme, _, ok := strings.Cut(uri, "://")
	if !ok || scheme == "" {
		return false
	}
	for i := range len(scheme) {
		c := scheme[i]
		isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if i == 0 {
			if !isAlpha {
				return false
			}
			continue
		}
		isRest := isAlpha || (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.'
		if !isRest {
			return false
		}
	}
	return true
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
