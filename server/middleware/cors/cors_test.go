package cors

import (
	"testing"

	"github.com/stretchr/testify/require"
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
			require.Equal(t, tt.want, cfg.resolveOrigin(tt.request))
		})
	}
}

func TestResolveOriginWildcardCredentials(t *testing.T) {
	t.Parallel()
	cfg := &config{allowOrigins: []string{"*"}}

	cfg.allowCredentials = false
	require.Equal(t, "*", cfg.resolveOrigin("https://example.com"))

	cfg.allowCredentials = true
	require.Equal(t, "https://example.com", cfg.resolveOrigin("https://example.com"))
}
