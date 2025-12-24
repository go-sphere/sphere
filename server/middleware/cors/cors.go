package cors

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-sphere/httpx"
)

const defaultAllowHeaders = "Origin,Content-Type,Accept,Authorization"

// Option configures the behavior of the CORS middleware.
type Option func(*config)

// WithAllowOrigins sets the list of allowed origins. Use "*" to allow any origin.
func WithAllowOrigins(origins ...string) Option {
	return func(cfg *config) {
		cfg.allowOrigins = copyStrings(origins)
	}
}

// WithAllowMethods sets the HTTP methods that are allowed for CORS requests.
func WithAllowMethods(methods ...string) Option {
	return func(cfg *config) {
		cfg.allowMethods = copyStrings(methods)
	}
}

// WithAllowHeaders sets the allowed request headers for preflight requests.
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
func WithMaxAge(ttl time.Duration) Option {
	return func(cfg *config) {
		cfg.maxAge = ttl
	}
}

// NewCORS creates an httpx middleware that applies configurable CORS headers.
// By default it allows all origins, standard HTTP verbs, and reflects requested headers.
func NewCORS(options ...Option) httpx.Middleware {
	cfg := newConfig(options...)
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
		return ctx.Next()
	}
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
	cfg := &config{
		allowOrigins: []string{""},
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
	if len(c.allowOrigins) == 0 {
		c.allowOrigins = []string{""}
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
		if seconds < 0 {
			seconds = 0
		}
		c.maxAgeValue = strconv.FormatInt(int64(seconds), 10)
		c.hasMaxAge = true
	}
}

func (c *config) apply(method, origin, reqHeaders string, setHeader func(string, string)) bool {
	allowedOrigin := c.resolveOrigin(origin)
	if allowedOrigin != "" {
		setHeader("Access-Control-Allow-Origin", allowedOrigin)
		if allowedOrigin != "*" {
			setHeader("Vary", "Origin")
		}
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

func (c *config) resolveOrigin(requestOrigin string) string {
	if len(c.allowOrigins) == 0 {
		return ""
	}
	for _, allowed := range c.allowOrigins {
		if allowed == "*" {
			if c.allowCredentials && requestOrigin != "" {
				return requestOrigin
			}
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
