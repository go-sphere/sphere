package cors

import (
	"errors"
	"testing"
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
