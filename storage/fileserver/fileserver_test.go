package fileserver

import (
	"context"
	"io"
	"path"
	"strings"
	"testing"

	"github.com/go-sphere/sphere/cache/memory"
)

type noopStorage struct{}

func (noopStorage) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	return key, nil
}

func (noopStorage) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	return key, nil
}

func (noopStorage) IsFileExists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (noopStorage) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	return io.NopCloser(strings.NewReader("")), "application/octet-stream", 0, nil
}

func (noopStorage) DeleteFile(ctx context.Context, key string) error {
	return nil
}

func (noopStorage) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	return nil
}

func (noopStorage) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	return nil
}

func TestNewCDNAdapter_ValidateDependencies(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewCDNAdapter(nil, memory.NewByteCache(), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("nil cache", func(t *testing.T) {
		_, err := NewCDNAdapter(&Config{
			PutBase: "https://example.com",
			GetBase: "https://example.com",
		}, nil, noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("nil store", func(t *testing.T) {
		_, err := NewCDNAdapter(&Config{
			PutBase: "https://example.com",
			GetBase: "https://example.com",
		}, memory.NewByteCache(), nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty put base", func(t *testing.T) {
		_, err := NewCDNAdapter(&Config{GetBase: "https://example.com"}, memory.NewByteCache(), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty get base", func(t *testing.T) {
		_, err := NewCDNAdapter(&Config{PutBase: "https://example.com"}, memory.NewByteCache(), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGenerateUploadToken_RejectNilNameBuilder(t *testing.T) {
	server, err := NewCDNAdapter(
		&Config{
			PutBase: "https://example.com",
			GetBase: "https://example.com",
		},
		memory.NewByteCache(),
		noopStorage{},
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}

	_, err = server.GenerateUploadToken(context.Background(), "avatar.png", "test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGenerateUploadToken_UsesSeparatedPutAndGetBase(t *testing.T) {
	server, err := NewCDNAdapter(
		&Config{
			PutBase: "https://upload.example.com",
			GetBase: "https://cdn.example.com",
		},
		memory.NewByteCache(),
		noopStorage{},
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}

	result, err := server.GenerateUploadToken(context.Background(), "avatar.png", "users", func(filename string, dir ...string) string {
		return path.Join(append(dir, filename)...)
	})
	if err != nil {
		t.Fatalf("GenerateUploadToken() error = %v", err)
	}

	if !strings.HasPrefix(result[0], "https://upload.example.com/") {
		t.Fatalf("uploadURL = %q, want prefix %q", result[0], "https://upload.example.com/")
	}
	if result[1] != "users/avatar.png" {
		t.Fatalf("key = %q, want %q", result[1], "users/avatar.png")
	}
	if result[2] != "https://cdn.example.com/users/avatar.png" {
		t.Fatalf("publicURL = %q, want %q", result[2], "https://cdn.example.com/users/avatar.png")
	}
}

func TestNormalizeWildcardParam(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "leading slash", in: "/a/b/c.png", want: "a/b/c.png"},
		{name: "no leading slash", in: "a/b/c.png", want: "a/b/c.png"},
		{name: "root slash", in: "/", want: ""},
		{name: "empty", in: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeWildcardParam(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeWildcardParam(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
