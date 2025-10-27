package urlhandler

import (
	"net/url"
	"testing"
)

func TestNewHandler(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		wantErr   bool
		wantBase  string
		wantHost  string
		wantPath  string
	}{
		{
			name:      "valid HTTP URL",
			publicURL: "http://localhost:8080",
			wantErr:   false,
			wantBase:  "http://localhost:8080",
			wantHost:  "localhost:8080",
			wantPath:  "",
		},
		{
			name:      "valid HTTPS URL",
			publicURL: "https://cdn.example.com",
			wantErr:   false,
			wantBase:  "https://cdn.example.com",
			wantHost:  "cdn.example.com",
			wantPath:  "",
		},
		{
			name:      "URL with trailing slash",
			publicURL: "http://localhost:8080/",
			wantErr:   false,
			wantBase:  "http://localhost:8080",
			wantHost:  "localhost:8080",
			wantPath:  "",
		},
		{
			name:      "URL with base path",
			publicURL: "http://localhost:8080/storage",
			wantErr:   false,
			wantBase:  "http://localhost:8080/storage",
			wantHost:  "localhost:8080",
			wantPath:  "storage",
		},
		{
			name:      "invalid URL",
			publicURL: "://invalid",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewHandler(tt.publicURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if handler.publicURLBase != tt.wantBase {
					t.Errorf("publicURLBase = %v, want %v", handler.publicURLBase, tt.wantBase)
				}
				if handler.publicURL.Host != tt.wantHost {
					t.Errorf("publicURL.Host = %v, want %v", handler.publicURL.Host, tt.wantHost)
				}
				if handler.basePath != tt.wantPath {
					t.Errorf("basePath = %v, want %v", handler.basePath, tt.wantPath)
				}
			}
		})
	}
}

func TestHandler_GenerateURL(t *testing.T) {
	handler, err := NewHandler("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "simple key",
			key:  "test.jpg",
			want: "http://localhost:8080/test.jpg",
		},
		{
			name: "key with path",
			key:  "images/test.jpg",
			want: "http://localhost:8080/images/test.jpg",
		},
		{
			name: "key with leading slash",
			key:  "/test.jpg",
			want: "http://localhost:8080/test.jpg",
		},
		{
			name: "empty key",
			key:  "",
			want: "",
		},
		{
			name: "key is already full URL",
			key:  "http://other.com/test.jpg",
			want: "http://other.com/test.jpg",
		},
		{
			name: "key is HTTPS URL",
			key:  "https://other.com/test.jpg",
			want: "https://other.com/test.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.GenerateURL(tt.key)
			if got != tt.want {
				t.Errorf("GenerateURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandler_GenerateURLs(t *testing.T) {
	handler, err := NewHandler("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		keys []string
		want []string
	}{
		{
			name: "multiple keys",
			keys: []string{"test1.jpg", "test2.jpg", "images/test3.jpg"},
			want: []string{
				"http://localhost:8080/test1.jpg",
				"http://localhost:8080/test2.jpg",
				"http://localhost:8080/images/test3.jpg",
			},
		},
		{
			name: "empty array",
			keys: []string{},
			want: []string{},
		},
		{
			name: "contains empty string",
			keys: []string{"test.jpg", "", "other.jpg"},
			want: []string{"http://localhost:8080/test.jpg", "", "http://localhost:8080/other.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.GenerateURLs(tt.keys)
			if len(got) != len(tt.want) {
				t.Errorf("GenerateURLs() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GenerateURLs()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHandler_ExtractKeyFromURLWithMode(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		uri       string
		strict    bool
		want      string
		wantErr   bool
	}{
		{
			name:      "full URL - strict mode",
			publicURL: "http://localhost:8080",
			uri:       "http://localhost:8080/test.jpg",
			strict:    true,
			want:      "test.jpg",
			wantErr:   false,
		},
		{
			name:      "URL with query params - strict mode",
			publicURL: "http://localhost:8080",
			uri:       "http://localhost:8080/test.jpg?width=100&height=200",
			strict:    true,
			want:      "test.jpg",
			wantErr:   false,
		},
		{
			name:      "URL with fragment - strict mode",
			publicURL: "http://localhost:8080",
			uri:       "http://localhost:8080/test.jpg#section",
			strict:    true,
			want:      "test.jpg",
			wantErr:   false,
		},
		{
			name:      "URL with path - strict mode",
			publicURL: "http://localhost:8080",
			uri:       "http://localhost:8080/images/test.jpg",
			strict:    true,
			want:      "images/test.jpg",
			wantErr:   false,
		},
		{
			name:      "wrong host - strict mode",
			publicURL: "http://localhost:8080",
			uri:       "http://other.com/test.jpg",
			strict:    true,
			want:      "",
			wantErr:   true,
		},
		{
			name:      "wrong host - non-strict mode",
			publicURL: "http://localhost:8080",
			uri:       "http://other.com/test.jpg",
			strict:    false,
			want:      "test.jpg",
			wantErr:   false,
		},
		{
			name:      "relative path",
			publicURL: "http://localhost:8080",
			uri:       "test.jpg",
			strict:    true,
			want:      "test.jpg",
			wantErr:   false,
		},
		{
			name:      "relative path with leading slash",
			publicURL: "http://localhost:8080",
			uri:       "/test.jpg",
			strict:    true,
			want:      "test.jpg",
			wantErr:   false,
		},
		{
			name:      "empty string",
			publicURL: "http://localhost:8080",
			uri:       "",
			strict:    true,
			want:      "",
			wantErr:   false,
		},
		{
			name:      "invalid URL",
			publicURL: "http://localhost:8080",
			uri:       "http://[invalid",
			strict:    false,
			want:      "",
			wantErr:   true,
		},
		{
			name:      "default port allowed in strict mode",
			publicURL: "https://cdn.example.com/static",
			uri:       "https://cdn.example.com:443/static/image.jpg",
			strict:    true,
			want:      "image.jpg",
			wantErr:   false,
		},
		{
			name:      "mismatched base path in strict mode",
			publicURL: "https://cdn.example.com/static",
			uri:       "https://cdn.example.com/staticx/image.jpg",
			strict:    true,
			want:      "",
			wantErr:   true,
		},
		{
			name:      "mismatched base path in non-strict mode",
			publicURL: "https://cdn.example.com/static",
			uri:       "https://cdn.example.com/staticx/image.jpg",
			strict:    false,
			want:      "staticx/image.jpg",
			wantErr:   false,
		},
		{
			name:      "encoded path is unescaped",
			publicURL: "https://cdn.example.com/static",
			uri:       "https://cdn.example.com/static/images%2Fphoto.jpg",
			strict:    true,
			want:      "images/photo.jpg",
			wantErr:   false,
		},
		{
			name:      "http scheme with explicit default port",
			publicURL: "http://localhost",
			uri:       "http://localhost:80/test.png",
			strict:    true,
			want:      "test.png",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewHandler(tt.publicURL)
			if err != nil {
				t.Fatalf("NewHandler() error = %v", err)
			}

			got, err := handler.ExtractKeyFromURLWithMode(tt.uri, tt.strict)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractKeyFromURLWithMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractKeyFromURLWithMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandler_ExtractKeyFromURL(t *testing.T) {
	handler, err := NewHandler("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "normal URL",
			uri:  "http://localhost:8080/test.jpg",
			want: "test.jpg",
		},
		{
			name: "URL with query params",
			uri:  "http://localhost:8080/test.jpg?width=100",
			want: "test.jpg",
		},
		{
			name: "wrong host (should return empty)",
			uri:  "http://other.com/test.jpg",
			want: "",
		},
		{
			name: "relative path",
			uri:  "test.jpg",
			want: "test.jpg",
		},
		{
			name: "empty string",
			uri:  "",
			want: "",
		},
		{
			name: "invalid URL (should return empty)",
			uri:  "http://[invalid",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.ExtractKeyFromURL(tt.uri)
			if got != tt.want {
				t.Errorf("ExtractKeyFromURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasHttpScheme(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want bool
	}{
		{
			name: "HTTP scheme",
			uri:  "http://example.com",
			want: true,
		},
		{
			name: "HTTPS scheme",
			uri:  "https://example.com",
			want: true,
		},
		{
			name: "relative path",
			uri:  "test.jpg",
			want: false,
		},
		{
			name: "absolute path",
			uri:  "/test.jpg",
			want: false,
		},
		{
			name: "FTP scheme",
			uri:  "ftp://example.com",
			want: false,
		},
		{
			name: "empty string",
			uri:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasHttpScheme(tt.uri)
			if got != tt.want {
				t.Errorf("hasHttpScheme() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSameHost(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		target   string
		wantSame bool
	}{
		{
			name:     "identical host without ports",
			baseURL:  "https://cdn.example.com",
			target:   "https://cdn.example.com/image.jpg",
			wantSame: true,
		},
		{
			name:     "identical host with explicit default port",
			baseURL:  "https://cdn.example.com",
			target:   "https://cdn.example.com:443/image.jpg",
			wantSame: true,
		},
		{
			name:     "base without port matches http default port",
			baseURL:  "http://localhost",
			target:   "http://localhost:80/image.jpg",
			wantSame: true,
		},
		{
			name:     "base with port requires explicit match",
			baseURL:  "http://localhost:8080",
			target:   "http://localhost:8080/image.jpg",
			wantSame: true,
		},
		{
			name:     "base with port does not match missing port",
			baseURL:  "http://localhost:8080",
			target:   "http://localhost/image.jpg",
			wantSame: false,
		},
		{
			name:     "different host",
			baseURL:  "https://cdn.example.com",
			target:   "https://other.example.com/image.jpg",
			wantSame: false,
		},
		{
			name:     "different port value",
			baseURL:  "https://cdn.example.com:8443",
			target:   "https://cdn.example.com:443/image.jpg",
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewHandler(tt.baseURL)
			if err != nil {
				t.Fatalf("NewHandler() error = %v", err)
			}
			targetURL, err := url.Parse(tt.target)
			if err != nil {
				t.Fatalf("url.Parse() error = %v", err)
			}

			got := sameHost(targetURL, handler.publicURL)
			if got != tt.wantSame {
				t.Errorf("sameHost() = %v, want %v", got, tt.wantSame)
			}
		})
	}
}

func TestHandler_WithBasePathURL(t *testing.T) {
	// Test URL handling with base path
	handler, err := NewHandler("http://localhost:8080/storage")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("generated URL should include base path", func(t *testing.T) {
		got := handler.GenerateURL("test.jpg")
		want := "http://localhost:8080/storage/test.jpg"
		if got != want {
			t.Errorf("GenerateURL() = %v, want %v", got, want)
		}
	})

	t.Run("extract key should handle base path correctly", func(t *testing.T) {
		got, err := handler.ExtractKeyFromURLWithMode("http://localhost:8080/storage/test.jpg", true)
		if err != nil {
			t.Fatal(err)
		}
		want := "test.jpg"
		if got != want {
			t.Errorf("ExtractKeyFromURLWithMode() = %v, want %v", got, want)
		}
	})
}
