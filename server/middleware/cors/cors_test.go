package cors

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
)

func TestResolveOriginWildcard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		allowOrigins []string
		request      string
		want         string
	}{
		{
			name:         "hostPortWildcard",
			allowOrigins: []string{"localhost:*"},
			request:      "http://localhost:3000",
			want:         "http://localhost:3000",
		},
		{
			name:         "schemeAwareWildcard",
			allowOrigins: []string{"https://*.example.com"},
			request:      "https://api.example.com",
			want:         "https://api.example.com",
		},
		{
			name:         "exactHostMatch",
			allowOrigins: []string{"localhost:4000"},
			request:      "http://localhost:4000",
			want:         "http://localhost:4000",
		},
		{
			name:         "noMatch",
			allowOrigins: []string{"https://*.example.com"},
			request:      "https://example.org",
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config{allowOrigins: tt.allowOrigins}
			got := cfg.resolveOrigin(tt.request)
			if got != tt.want {
				t.Fatalf("resolveOrigin(%q) = %q, want %q", tt.request, got, tt.want)
			}
		})
	}
}

// TestResolveOriginWildcardNeverReflects pins that a bare "*" resolves to the
// literal wildcard and never to the caller's own origin. Reflecting it back
// while Allow-Credentials is set would let any site read authenticated
// responses with the victim's cookies.
func TestResolveOriginWildcardNeverReflects(t *testing.T) {
	t.Parallel()
	cfg := &config{allowOrigins: []string{"*"}}

	for _, credentials := range []bool{false, true} {
		cfg.allowCredentials = credentials
		if got := cfg.resolveOrigin("https://evil.example"); got != "*" {
			t.Fatalf("resolveOrigin(credentials=%v) = %q, want %q", credentials, got, "*")
		}
	}
}

// TestNewCORSRejectsWildcardWithCredentials pins that the forbidden combination
// fails at construction instead of degrading into an allow-any-origin policy.
func TestNewCORSRejectsWildcardWithCredentials(t *testing.T) {
	t.Parallel()

	if _, err := NewCORS(WithAllowOrigins("*"), WithAllowCredentials(true)); !errors.Is(err, ErrWildcardWithCredentials) {
		t.Fatalf("expected ErrWildcardWithCredentials, got %v", err)
	}

	// A per-origin wildcard stays valid: it resolves to a single matched origin.
	if _, err := NewCORS(WithAllowOrigins("https://*.example.com"), WithAllowCredentials(true)); err != nil {
		t.Fatalf("per-origin wildcard with credentials must be allowed, got %v", err)
	}

	// "*" without credentials is the ordinary public-API configuration.
	if _, err := NewCORS(WithAllowOrigins("*")); err != nil {
		t.Fatalf("wildcard without credentials must be allowed, got %v", err)
	}
}

type httpxContext = httpx.Context

type fakeContext struct {
	httpxContext
	method      string
	headers     map[string]string
	respHeaders map[string]string
	status      int
	nextCalled  bool
}

func newFakeContext(method string, headers map[string]string) *fakeContext {
	if headers == nil {
		headers = make(map[string]string)
	}
	return &fakeContext{
		method:      method,
		headers:     headers,
		respHeaders: make(map[string]string),
	}
}

func (f *fakeContext) Method() string {
	return f.method
}

func (f *fakeContext) Header(key string) string {
	return f.headers[key]
}

func (f *fakeContext) SetHeader(key, value string) {
	f.respHeaders[key] = value
}

func (f *fakeContext) NoContent(code int) error {
	f.status = code
	return nil
}

func (f *fakeContext) Next() error {
	f.nextCalled = true
	return nil
}

func TestCORS_OptionsPreflight(t *testing.T) {
	t.Parallel()

	mw, err := NewCORS(
		WithAllowOrigins("https://example.com"),
		WithAllowMethods("GET", "POST", "PUT"),
		WithAllowHeaders("X-Custom-Header", "Authorization"),
		WithExposeHeaders("X-Exposed-1", "X-Exposed-2"),
		WithMaxAge(10*time.Minute),
	)
	if err != nil {
		t.Fatalf("NewCORS: %v", err)
	}

	ctx := newFakeContext(http.MethodOptions, map[string]string{
		"Origin":                         "https://example.com",
		"Access-Control-Request-Headers": "X-Custom-Header",
	})

	if err := mw(ctx); err != nil {
		t.Fatalf("mw(ctx): %v", err)
	}

	if ctx.status != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", ctx.status)
	}
	if ctx.nextCalled {
		t.Fatal("Next() should not be called on OPTIONS preflight")
	}
	if got := ctx.respHeaders["Access-Control-Allow-Origin"]; got != "https://example.com" {
		t.Fatalf("Allow-Origin = %q, want https://example.com", got)
	}
	if got := ctx.respHeaders["Vary"]; got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}
	if got := ctx.respHeaders["Access-Control-Allow-Methods"]; got != "GET,POST,PUT" {
		t.Fatalf("Allow-Methods = %q, want GET,POST,PUT", got)
	}
	if got := ctx.respHeaders["Access-Control-Allow-Headers"]; got != "X-Custom-Header,Authorization" {
		t.Fatalf("Allow-Headers = %q, want X-Custom-Header,Authorization", got)
	}
	if got := ctx.respHeaders["Access-Control-Expose-Headers"]; got != "X-Exposed-1,X-Exposed-2" {
		t.Fatalf("Expose-Headers = %q, want X-Exposed-1,X-Exposed-2", got)
	}
	if got := ctx.respHeaders["Access-Control-Max-Age"]; got != "600" {
		t.Fatalf("Max-Age = %q, want 600", got)
	}
}

func TestCORS_ActualGetRequest(t *testing.T) {
	t.Parallel()

	mw, err := NewCORS(
		WithAllowOrigins("https://example.com"),
		WithAllowMethods("GET", "POST"),
		WithExposeHeaders("X-Trace-Id"),
		WithAllowCredentials(true),
	)
	if err != nil {
		t.Fatalf("NewCORS: %v", err)
	}

	ctx := newFakeContext(http.MethodGet, map[string]string{
		"Origin": "https://example.com",
	})

	if err := mw(ctx); err != nil {
		t.Fatalf("mw(ctx): %v", err)
	}

	if !ctx.nextCalled {
		t.Fatal("Next() should be called on GET request")
	}
	if got := ctx.respHeaders["Access-Control-Allow-Origin"]; got != "https://example.com" {
		t.Fatalf("Allow-Origin = %q, want https://example.com", got)
	}
	if got := ctx.respHeaders["Access-Control-Allow-Credentials"]; got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}
	if got := ctx.respHeaders["Access-Control-Expose-Headers"]; got != "X-Trace-Id" {
		t.Fatalf("Expose-Headers = %q, want X-Trace-Id", got)
	}
	if got := ctx.respHeaders["Access-Control-Allow-Methods"]; got != "GET,POST" {
		t.Fatalf("Allow-Methods = %q, want GET,POST", got)
	}
}

func TestCORS_RequestHeadersFallback(t *testing.T) {
	t.Parallel()

	mw, err := NewCORS(WithAllowOrigins("https://example.com"))
	if err != nil {
		t.Fatalf("NewCORS: %v", err)
	}

	// 1. When request provides Access-Control-Request-Headers
	ctx1 := newFakeContext(http.MethodOptions, map[string]string{
		"Origin":                         "https://example.com",
		"Access-Control-Request-Headers": "X-Custom-1,X-Custom-2",
	})
	if err := mw(ctx1); err != nil {
		t.Fatalf("mw(ctx1): %v", err)
	}
	if got := ctx1.respHeaders["Access-Control-Allow-Headers"]; got != "X-Custom-1,X-Custom-2" {
		t.Fatalf("Allow-Headers = %q, want X-Custom-1,X-Custom-2", got)
	}

	// 2. When no request headers provided, fallback to defaultAllowHeaders
	ctx2 := newFakeContext(http.MethodOptions, map[string]string{
		"Origin": "https://example.com",
	})
	if err := mw(ctx2); err != nil {
		t.Fatalf("mw(ctx2): %v", err)
	}
	if got := ctx2.respHeaders["Access-Control-Allow-Headers"]; got != defaultAllowHeaders {
		t.Fatalf("Allow-Headers = %q, want default %q", got, defaultAllowHeaders)
	}
}

func TestCORS_UnmatchedOrigin(t *testing.T) {
	t.Parallel()

	mw, err := NewCORS(WithAllowOrigins("https://example.com"))
	if err != nil {
		t.Fatalf("NewCORS: %v", err)
	}

	ctx := newFakeContext(http.MethodGet, map[string]string{
		"Origin": "https://unauthorized.com",
	})

	if err := mw(ctx); err != nil {
		t.Fatalf("mw(ctx): %v", err)
	}

	if !ctx.nextCalled {
		t.Fatal("Next() should be called on GET request")
	}
	if got := ctx.respHeaders["Access-Control-Allow-Origin"]; got != "" {
		t.Fatalf("Allow-Origin = %q, want empty", got)
	}
}
