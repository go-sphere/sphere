package fileserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage"
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

func (noopStorage) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	return storage.DownloadResult{
		Reader: io.NopCloser(strings.NewReader("")),
		MIME:   "application/octet-stream",
		Size:   0,
	}, nil
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
	t.Run("empty config", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{}, memory.NewByteCache(), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("nil cache", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{
			PutBase: "https://example.com",
			GetBase: "https://example.com",
		}, nil, noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("nil store", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{
			PutBase: "https://example.com",
			GetBase: "https://example.com",
		}, memory.NewByteCache(), nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty put base", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{GetBase: "https://example.com"}, memory.NewByteCache(), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty get base", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{PutBase: "https://example.com"}, memory.NewByteCache(), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestFileServerCloseOwnsCacheOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	newAdapter := func(t *testing.T, c cache.ByteCache, opts ...Option) *FileServer {
		t.Helper()
		a, err := NewCDNAdapter(Config{
			PutBase: "https://example.com/put",
			GetBase: "https://example.com",
		}, c, noopStorage{}, opts...)
		if err != nil {
			t.Fatalf("NewCDNAdapter: %v", err)
		}
		return a
	}

	t.Run("owned cache is closed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		c := memory.NewByteCache()
		adapter := newAdapter(t, c, WithOwnedCache())

		// Repeated and concurrent Close calls must all succeed: a task's Stop
		// can run more than once, and tasktest exercises concurrent Stops.
		const closers = 8
		errs := make(chan error, closers)
		var wg sync.WaitGroup
		wg.Add(closers)
		for range closers {
			go func() {
				defer wg.Done()
				errs <- adapter.Close()
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("Close() = %v, want nil", err)
			}
		}

		if err := c.Set(ctx, "token", []byte("key")); !errors.Is(err, cache.ErrClosed) {
			t.Fatalf("cache.Set after Close = %v, want cache.ErrClosed", err)
		}
		if err := adapter.Close(); err != nil {
			t.Fatalf("Close() after Close = %v, want nil", err)
		}
	})

	t.Run("injected cache is left alone", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		c := memory.NewByteCache()
		defer func() { _ = c.Close() }()
		adapter := newAdapter(t, c)

		if err := adapter.Close(); err != nil {
			t.Fatalf("Close() = %v, want nil", err)
		}
		if err := c.Set(ctx, "token", []byte("key")); err != nil {
			t.Fatalf("cache.Set = %v, want nil: the caller still owns the cache", err)
		}
	})
}

// successContext captures the JSON response body so the envelope can be
// asserted. httpxContext aliases httpx.Context so embedding it does not create
// a field named Context, which would shadow the interface's Context() method.
type successContext struct {
	httpxContext
	body []byte
}

func (s *successContext) JSON(code int, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.body = b
	return nil
}

type httpxContext = httpx.Context

// TestUploadSuccessEnvelopeHasSuccessTrue pins that the default upload success
// writer reports the envelope's Success field as true. DataResponse.Success
// serializes without omitempty, so forgetting to set it made every successful
// upload come back as success:false.
func TestUploadSuccessEnvelopeHasSuccessTrue(t *testing.T) {
	ctx := &successContext{}
	if err := defaultUploadSuccessWithData(ctx, "abc", "https://example.com/abc"); err != nil {
		t.Fatalf("defaultUploadSuccessWithData: %v", err)
	}
	var resp struct {
		Success bool         `json:"success"`
		Data    UploadResult `json:"data"`
	}
	if err := json.Unmarshal(ctx.body, &resp); err != nil {
		t.Fatalf("unmarshal body %q: %v", ctx.body, err)
	}
	if !resp.Success {
		t.Fatalf("success = false, want true (body %s)", ctx.body)
	}
	if resp.Data.Key != "abc" || resp.Data.URL != "https://example.com/abc" {
		t.Fatalf("data = %+v, want key=abc url=https://example.com/abc", resp.Data)
	}
}

func TestGenerateUploadAuth_RejectEmptyFileName(t *testing.T) {
	server, err := NewCDNAdapter(
		Config{
			PutBase: "https://example.com",
			GetBase: "https://example.com",
		},
		memory.NewByteCache(),
		noopStorage{},
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}

	_, err = server.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		FileName: "",
		Dir:      "test",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGenerateUploadAuth_UsesSeparatedPutAndGetBase(t *testing.T) {
	server, err := NewCDNAdapter(
		Config{
			PutBase:      "https://upload.example.com",
			GetBase:      "https://cdn.example.com",
			Dir:          "base",
			UploadNaming: storage.UploadNamingStrategyOriginal,
		},
		memory.NewByteCache(),
		noopStorage{},
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}

	result, err := server.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		FileName: "avatar.png",
		Dir:      "users",
	})
	if err != nil {
		t.Fatalf("GenerateUploadAuth() error = %v", err)
	}

	if !strings.HasPrefix(result.Authorization.Value, "https://upload.example.com/") {
		t.Fatalf("uploadURL = %q, want prefix %q", result.Authorization.Value, "https://upload.example.com/")
	}
	if result.Authorization.Type != storage.UploadAuthorizationTypeURL {
		t.Fatalf("auth type = %q, want %q", result.Authorization.Type, storage.UploadAuthorizationTypeURL)
	}
	if result.File.Key != "base/users/avatar.png" {
		t.Fatalf("key = %q, want %q", result.File.Key, "base/users/avatar.png")
	}
	if result.File.URL != "https://cdn.example.com/base/users/avatar.png" {
		t.Fatalf("publicURL = %q, want %q", result.File.URL, "https://cdn.example.com/base/users/avatar.png")
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
