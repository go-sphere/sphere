package cors

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-sphere/httpx"
)

const defaultAllowHeaders = "Origin,Content-Type,Accept,Authorization"

// Option configures the behavior of the CORS middleware.
type Option func(*config)

// WithAllowOrigins sets the list of allowed origins. Use "*" to allow any origin.
//
// An entry containing "://" is compared with the full request origin; any
// other entry is compared with the origin's host (including port). Matching is
// case-insensitive, and "*" inside an entry matches any run of characters, as
// in "https://*.example.com". A matching origin is echoed in
// Access-Control-Allow-Origin. The default is no origins. A later call replaces
// the list.
func WithAllowOrigins(origins ...string) Option {
	return func(cfg *config) {
		cfg.allowOrigins = slices.Clone(origins)
	}
}

// WithAllowMethods sets the HTTP methods that are allowed for CORS requests.
// It replaces the default GET, POST, PUT, DELETE, PATCH, OPTIONS; an empty list
// advertises OPTIONS only.
func WithAllowMethods(methods ...string) Option {
	return func(cfg *config) {
		cfg.allowMethods = slices.Clone(methods)
	}
}

// WithAllowHeaders sets the allowed request headers for preflight requests.
// Without it the middleware echoes Access-Control-Request-Headers, or sends
// "Origin,Content-Type,Accept,Authorization" when the request names none.
func WithAllowHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.allowHeaders = slices.Clone(headers)
	}
}

// WithExposeHeaders defines which headers are exposed to browser clients.
func WithExposeHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.exposeHeaders = slices.Clone(headers)
	}
}

// WithAllowCredentials enables or disables the Access-Control-Allow-Credentials header.
func WithAllowCredentials(enabled bool) Option {
	return func(cfg *config) {
		cfg.allowCredentials = enabled
	}
}

// WithMaxAge sets how long the results of a preflight request can be cached.
// It is sent as whole seconds (fractions are truncated). Zero or negative
// omits Access-Control-Max-Age.
func WithMaxAge(ttl time.Duration) Option {
	return func(cfg *config) {
		cfg.maxAge = ttl
	}
}

// ErrWildcardWithCredentials rejects unbounded wildcard origins paired with AllowCredentials.
// The Fetch standard forbids "*" + AllowCredentials: true, and reflecting the
// request Origin on an unbounded wildcard pattern ("*", "**", "https://*",
// "*://*", "*:*", "*.*") turns the middleware into an allow-any-origin-with-cookies
// policy. List explicit origins or constrain the wildcard to a concrete host
// such as "https://*.example.com" or "http://localhost:*".
var ErrWildcardWithCredentials = errors.New("cors: \"*\" cannot be combined with AllowCredentials; list explicit origins")

// NewCORS creates an httpx middleware that applies configurable CORS headers.
// By default it allows standard HTTP verbs and reflects requested headers.
// Allowed origins should be explicitly configured via WithAllowOrigins (or "*" for public APIs).
// It returns ErrWildcardWithCredentials when the configuration pairs credentials
// with "*" or another unbounded wildcard origin pattern, so the misconfiguration
// surfaces at startup rather than becoming a runtime hole.
//
// Options are applied in order (nil options are skipped) and copied at
// construction. Every OPTIONS request is answered with 204 without calling
// the next handler; other requests get the CORS headers and continue.
func NewCORS(options ...Option) (httpx.Middleware, error) {
	cfg := newConfig(options...)
	if cfg.allowCredentials && cfg.hasUnboundedWildcardOrigin() {
		return nil, ErrWildcardWithCredentials
	}
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			setHeader := ctx.SetHeader
			if editor, ok := httpx.AsResponseHeaderEditor(ctx); ok {
				// Vary is a list: replacing it would drop what an outer
				// middleware (compression's Accept-Encoding) already added.
				setHeader = func(key, value string) {
					if key == "Vary" {
						editor.AddHeader(key, value)
						return
					}
					ctx.SetHeader(key, value)
				}
			}
			preflight := cfg.apply(
				ctx.Method(),
				ctx.Header("Origin"),
				ctx.Header("Access-Control-Request-Headers"),
				setHeader,
			)
			if preflight {
				return ctx.NoContent(http.StatusNoContent)
			}
			return next(ctx)
		}
	}, nil
}

type compiledOrigin struct {
	raw         string
	lower       string
	hasScheme   bool
	hasWildcard bool
}

type config struct {
	allowOrigins     []string
	allowMethods     []string
	allowHeaders     []string
	exposeHeaders    []string
	allowCredentials bool
	maxAge           time.Duration

	compiledOrigins    []compiledOrigin
	allowMethodsValue  string
	allowHeadersValue  string
	exposeHeadersValue string
	maxAgeValue        string

	hasAllowHeaders  bool
	hasExposeHeaders bool
	hasMaxAge        bool
}

func newConfig(options ...Option) *config {
	// Origins default to empty, which matches nothing; configure WithAllowOrigins.
	cfg := &config{
		allowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodPatch,
			http.MethodOptions,
		},
	}
	for _, opt := range options {
		if opt != nil {
			opt(cfg)
		}
	}
	cfg.compile()
	return cfg
}

func compileOrigin(allowed string) compiledOrigin {
	return compiledOrigin{
		raw:         allowed,
		lower:       strings.ToLower(allowed),
		hasScheme:   strings.Contains(allowed, "://"),
		hasWildcard: strings.Contains(allowed, "*"),
	}
}

func (c *config) compile() {
	c.compiledOrigins = make([]compiledOrigin, len(c.allowOrigins))
	for i, origin := range c.allowOrigins {
		c.compiledOrigins[i] = compileOrigin(origin)
	}
	if len(c.allowMethods) == 0 {
		c.allowMethods = []string{http.MethodOptions}
	}
	c.allowMethodsValue = strings.Join(c.allowMethods, ",")
	c.hasAllowHeaders = len(c.allowHeaders) > 0
	if c.hasAllowHeaders {
		c.allowHeadersValue = strings.Join(c.allowHeaders, ",")
	}
	c.hasExposeHeaders = len(c.exposeHeaders) > 0
	if c.hasExposeHeaders {
		c.exposeHeadersValue = strings.Join(c.exposeHeaders, ",")
	}
	if c.maxAge > 0 {
		seconds := c.maxAge / time.Second
		c.maxAgeValue = strconv.FormatInt(int64(seconds), 10)
		c.hasMaxAge = true
	}
}

func (c *config) apply(method, origin, reqHeaders string, setHeader func(string, string)) bool {
	allowedOrigin := c.resolveOrigin(origin)
	if !c.hasWildcardOrigin() {
		// Vary must be set even when the origin misses or is absent: the
		// response for this URL still varies on Origin, and a shared cache
		// that stores a CORS-header-less variant would replay it to an
		// allowed origin, breaking CORS for legitimate clients.
		setHeader("Vary", "Origin")
	}
	if allowedOrigin != "" {
		setHeader("Access-Control-Allow-Origin", allowedOrigin)
		if c.allowCredentials {
			setHeader("Access-Control-Allow-Credentials", "true")
		}
	}
	setHeader("Access-Control-Allow-Methods", c.allowMethodsValue)
	if c.hasAllowHeaders {
		setHeader("Access-Control-Allow-Headers", c.allowHeadersValue)
	} else if reqHeaders != "" {
		setHeader("Vary", "Access-Control-Request-Headers")
		setHeader("Access-Control-Allow-Headers", reqHeaders)
	} else {
		setHeader("Access-Control-Allow-Headers", defaultAllowHeaders)
	}
	if c.hasExposeHeaders {
		setHeader("Access-Control-Expose-Headers", c.exposeHeadersValue)
	}
	if c.hasMaxAge {
		setHeader("Access-Control-Max-Age", c.maxAgeValue)
	}
	return method == http.MethodOptions
}

// hasWildcardOrigin reports whether a bare "*" is configured. Per-origin
// wildcard patterns ("https://*.example.com") are not affected: those resolve to
// the matched request origin, which is a valid credentialed response.
func (c *config) hasWildcardOrigin() bool {
	return slices.Contains(c.allowOrigins, "*")
}

// hasUnboundedWildcardOrigin reports whether any configured origin contains a
// wildcard without any fixed (wildcard-free) domain label in its host.
func (c *config) hasUnboundedWildcardOrigin() bool {
	return slices.ContainsFunc(c.allowOrigins, isUnboundedWildcardOrigin)
}

func isUnboundedWildcardOrigin(allowed string) bool {
	if !strings.Contains(allowed, "*") {
		return false
	}
	if allowed == "*" {
		return true
	}
	rest := allowed
	if idx := strings.Index(rest, "://"); idx != -1 {
		rest = rest[idx+3:]
	}
	if slash := strings.IndexByte(rest, '/'); slash != -1 {
		rest = rest[:slash]
	}
	var host string
	if strings.HasPrefix(rest, "[") {
		end := strings.IndexByte(rest, ']')
		if end == -1 {
			return true
		}
		host = rest[:end+1]
	} else if colon := strings.LastIndexByte(rest, ':'); colon != -1 {
		host = rest[:colon]
	} else {
		host = rest
	}
	if host == "" {
		return true
	}
	for label := range strings.SplitSeq(host, ".") {
		if label != "" && !strings.Contains(label, "*") {
			return false
		}
	}
	return true
}

func (c *config) resolveOrigin(requestOrigin string) string {
	lowerOrigin, lowerHost, validOrigin := parseOrigin(requestOrigin)
	if len(c.compiledOrigins) > 0 {
		for _, allowed := range c.compiledOrigins {
			if allowed.raw == "*" {
				// Always the literal wildcard: NewCORS rejects "*" together with
				// credentials, so there is no case left where reflecting the request
				// origin would be correct.
				return "*"
			}
			if validOrigin && matchCompiledOrigin(lowerOrigin, lowerHost, allowed) {
				return requestOrigin
			}
		}
		return ""
	}
	for _, allowed := range c.allowOrigins {
		if allowed == "*" {
			return "*"
		}
		if validOrigin && matchCompiledOrigin(lowerOrigin, lowerHost, compileOrigin(allowed)) {
			return requestOrigin
		}
	}
	return ""
}

func matchCompiledOrigin(lowerOrigin, lowerHost string, allowed compiledOrigin) bool {
	if allowed.lower == "" {
		return false
	}
	candidate := lowerHost
	if allowed.hasScheme {
		candidate = lowerOrigin
	}
	if allowed.hasWildcard {
		return wildcardMatch(candidate, allowed.lower)
	}
	return candidate == allowed.lower
}

func wildcardMatch(value, pattern string) bool {
	if pattern == "" {
		return false
	}
	valueIdx, patternIdx := 0, 0
	starIdx, matchIdx := -1, 0

	for valueIdx < len(value) {
		switch {
		case patternIdx < len(pattern) && pattern[patternIdx] == '*':
			starIdx = patternIdx
			matchIdx = valueIdx
			patternIdx++
		case patternIdx < len(pattern) && pattern[patternIdx] == value[valueIdx]:
			valueIdx++
			patternIdx++
		case starIdx != -1:
			patternIdx = starIdx + 1
			matchIdx++
			valueIdx = matchIdx
		default:
			return false
		}
	}
	for patternIdx < len(pattern) && pattern[patternIdx] == '*' {
		patternIdx++
	}
	return patternIdx == len(pattern)
}

func parseOrigin(origin string) (lowerOrigin, lowerHost string, ok bool) {
	if origin == "" {
		return "", "", false
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", "", false
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", "", false
	}
	if parsed.User != nil || parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", "", false
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), strings.ToLower(parsed.Host), true
}
