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
		cfg.allowOrigins = copyStrings(origins)
	}
}

// WithAllowMethods sets the HTTP methods that are allowed for CORS requests.
// It replaces the default GET, POST, PUT, DELETE, PATCH, OPTIONS; an empty list
// advertises OPTIONS only.
func WithAllowMethods(methods ...string) Option {
	return func(cfg *config) {
		cfg.allowMethods = copyStrings(methods)
	}
}

// WithAllowHeaders sets the allowed request headers for preflight requests.
// Without it the middleware echoes Access-Control-Request-Headers, or sends
// "Origin,Content-Type,Accept,Authorization" when the request names none.
func WithAllowHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.allowHeaders = copyStrings(headers)
	}
}

// WithExposeHeaders defines which headers are exposed to browser clients.
func WithExposeHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.exposeHeaders = copyStrings(headers)
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

// ErrWildcardWithCredentials rejects the "*" + AllowCredentials combination.
// The Fetch standard forbids it: a browser refuses a wildcard Allow-Origin on a
// credentialed request. Reflecting the request origin back would "work", but it
// turns the configuration into an allow-any-origin-with-cookies policy, which is
// exactly what "*" is meant to prevent. List the origins explicitly instead —
// per-origin wildcards such as "https://*.example.com" remain valid.
var ErrWildcardWithCredentials = errors.New("cors: \"*\" cannot be combined with AllowCredentials; list explicit origins")

// NewCORS creates an httpx middleware that applies configurable CORS headers.
// By default it allows standard HTTP verbs and reflects requested headers.
// Allowed origins should be explicitly configured via WithAllowOrigins (or "*" for public APIs).
// It returns ErrWildcardWithCredentials when the configuration pairs a bare "*"
// origin with credentials, so the misconfiguration surfaces at startup rather
// than becoming a runtime hole.
//
// Options are applied in order (nil options are skipped) and copied at
// construction. Every OPTIONS request is answered with 204 without calling
// the next handler; other requests get the CORS headers and continue.
func NewCORS(options ...Option) (httpx.Middleware, error) {
	cfg := newConfig(options...)
	if cfg.allowCredentials && cfg.hasWildcardOrigin() {
		return nil, ErrWildcardWithCredentials
	}
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			preflight := cfg.apply(
				ctx.Method(),
				ctx.Header("Origin"),
				ctx.Header("Access-Control-Request-Headers"),
				ctx.SetHeader,
			)
			if preflight {
				return ctx.NoContent(http.StatusNoContent)
			}
			return next(ctx)
		}
	}, nil
}

type config struct {
	allowOrigins     []string
	allowMethods     []string
	allowHeaders     []string
	exposeHeaders    []string
	allowCredentials bool
	maxAge           time.Duration

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

func (c *config) compile() {
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

func (c *config) resolveOrigin(requestOrigin string) string {
	for _, allowed := range c.allowOrigins {
		if allowed == "*" {
			// Always the literal wildcard: NewCORS rejects "*" together with
			// credentials, so there is no case left where reflecting the request
			// origin would be correct.
			return "*"
		}
		if originMatches(requestOrigin, allowed) {
			return requestOrigin
		}
	}
	return ""
}

func originMatches(requestOrigin, allowed string) bool {
	if requestOrigin == "" || allowed == "" {
		return false
	}
	if strings.Contains(allowed, "://") {
		if strings.Contains(allowed, "*") {
			return wildcardMatch(strings.ToLower(requestOrigin), strings.ToLower(allowed))
		}
		return strings.EqualFold(requestOrigin, allowed)
	}

	host := extractHost(requestOrigin)
	if host == "" {
		return false
	}
	if strings.Contains(allowed, "*") {
		return wildcardMatch(strings.ToLower(host), strings.ToLower(allowed))
	}
	return strings.EqualFold(host, allowed)
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

func extractHost(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
