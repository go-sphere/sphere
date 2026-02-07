package cors

import (
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
		tt := tt
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

func TestResolveOriginWildcardCredentials(t *testing.T) {
	t.Parallel()
	cfg := &config{allowOrigins: []string{"*"}}

	cfg.allowCredentials = false
	if got := cfg.resolveOrigin("https://example.com"); got != "*" {
		t.Fatalf("resolveOrigin with credentials disabled = %q, want %q", got, "*")
	}

	cfg.allowCredentials = true
	if got := cfg.resolveOrigin("https://example.com"); got != "https://example.com" {
		t.Fatalf("resolveOrigin with credentials enabled = %q, want %q", got, "https://example.com")
	}
}
