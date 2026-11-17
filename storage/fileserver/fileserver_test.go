package fileserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
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
	newCache := func(t *testing.T) cache.ByteCache {
		t.Helper()
		c := memory.NewByteCache()
		t.Cleanup(func() { _ = c.Close() })
		return c
	}

	t.Run("empty config", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{}, newCache(t), noopStorage{})
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
		}, newCache(t), nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty put base", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{GetBase: "https://example.com"}, newCache(t), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty get base", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{PutBase: "https://example.com"}, newCache(t), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("relative get base", func(t *testing.T) {
		_, err := NewCDNAdapter(Config{PutBase: "https://example.com", GetBase: "/files"}, newCache(t), noopStorage{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("owned cache closed on error", func(t *testing.T) {
		c := memory.NewByteCache()
		_, err := NewCDNAdapter(Config{PutBase: "https://example.com", GetBase: "/files"}, c, noopStorage{}, nil, WithOwnedCache())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err := c.Set(context.Background(), "k", []byte("v")); !errors.Is(err, cache.ErrClosed) {
			t.Fatalf("cache.Set after failed NewCDNAdapter = %v, want cache.ErrClosed", err)
		}
	})

	t.Run("nil option ignored and negative key ttl normalized", func(t *testing.T) {
		c := newCache(t)
		adapter, err := NewCDNAdapter(Config{
			PutBase: "https://example.com/put",
			GetBase: "https://example.com/get",
			KeyTTL:  -time.Second,
		}, c, noopStorage{}, nil)
		if err != nil {
			t.Fatalf("NewCDNAdapter: %v", err)
		}
		if adapter.config.KeyTTL != defaultKeyTTL {
			t.Fatalf("KeyTTL = %v, want %v", adapter.config.KeyTTL, defaultKeyTTL)
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

// TestUploadSuccessEnvelopeHasSuccessTrue pins that the default upload success
// writer reports the envelope's Success field as true. DataResponse.Success
// serializes without omitempty, so forgetting to set it made every successful
// upload come back as success:false.
func TestUploadSuccessEnvelopeHasSuccessTrue(t *testing.T) {
	ctx := httpxmock.New(nil)
	if err := defaultUploadSuccessWithData(ctx, "abc", "https://example.com/abc"); err != nil {
		t.Fatalf("defaultUploadSuccessWithData: %v", err)
	}
	body := ctx.Body()
	var resp struct {
		Success bool         `json:"success"`
		Data    UploadResult `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal body %q: %v", body, err)
	}
	if !resp.Success {
		t.Fatalf("success = false, want true (body %s)", body)
	}
	if resp.Data.Key != "abc" || resp.Data.URL != "https://example.com/abc" {
		t.Fatalf("data = %+v, want key=abc url=https://example.com/abc", resp.Data)
	}
}

func TestGenerateUploadAuth_RejectEmptyFileName(t *testing.T) {
	c := memory.NewByteCache()
	t.Cleanup(func() { _ = c.Close() })
	server, err := NewCDNAdapter(
		Config{
			PutBase: "https://example.com",
			GetBase: "https://example.com",
		},
		c,
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
	c := memory.NewByteCache()
	t.Cleanup(func() { _ = c.Close() })
	server, err := NewCDNAdapter(
		Config{
			PutBase:      "https://upload.example.com",
			GetBase:      "https://cdn.example.com",
			Dir:          "base",
			UploadNaming: storage.UploadNamingStrategyOriginal,
		},
		c,
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

// TestUploadTokensAreNamespaced pins that the token cache the public PUT route
// redeems from cannot reach other entries of a shared cache: a URL segment
// naming an unrelated key must neither read nor delete it.
func TestUploadTokensAreNamespaced(t *testing.T) {
	ctx := context.Background()
	shared := mcache.NewByteCache()
	t.Cleanup(func() { _ = shared.Close() })
	if err := shared.Set(ctx, "session:abc", []byte("victim")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	server, err := NewCDNAdapter(Config{PutBase: "https://upload.example.com", GetBase: "https://cdn.example.com"}, shared, noopStorage{},
		WithCreateFileKey(func(context.Context) (string, error) { return "tok", nil }))
	if err != nil {
		t.Fatalf("NewCDNAdapter: %v", err)
	}

	if _, found, err := server.cache.GetDel(ctx, "session:abc"); err != nil || found {
		t.Fatalf("GetDel(session:abc) found=%v err=%v, want not found", found, err)
	}
	if _, found, _ := shared.Get(ctx, "session:abc"); !found {
		t.Fatal("unrelated shared entry was deleted")
	}

	result, err := server.GenerateUploadAuth(ctx, storage.UploadAuthRequest{FileName: "a.png"})
	if err != nil {
		t.Fatalf("GenerateUploadAuth: %v", err)
	}
	if _, found, _ := shared.Get(ctx, "tok"); found {
		t.Fatal("token stored without namespace")
	}
	key, found, err := server.cache.GetDel(ctx, "tok")
	if err != nil || !found || string(key) != result.File.Key {
		t.Fatalf("redeem token = %q found=%v err=%v, want %q", key, found, err, result.File.Key)
	}
}

func TestGenerateUploadAuth_RejectsInvalidCustomTokens(t *testing.T) {
	for _, badToken := range []string{"", ".", "..", "a/b", "a\\b", "tok?x=1", "tok#frag", "bad tok", "tok\x00"} {
		c := mcache.NewByteCache()
		server, err := NewCDNAdapter(
			Config{PutBase: "https://upload.example.com", GetBase: "https://cdn.example.com"},
			c,
			noopStorage{},
			WithCreateFileKey(func(context.Context) (string, error) { return badToken, nil }),
		)
		if err != nil {
			_ = c.Close()
			t.Fatalf("NewCDNAdapter(%q): %v", badToken, err)
		}
		if _, err := server.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{FileName: "a.png"}); err == nil {
			_ = c.Close()
			t.Fatalf("GenerateUploadAuth(token=%q) succeeded, want error", badToken)
		}
		if keys, err := c.Keys(context.Background(), "*"); err != nil || len(keys) != 0 {
			_ = c.Close()
			t.Fatalf("Keys after rejected token %q = %v (err=%v), want empty", badToken, keys, err)
		}
		_ = c.Close()
	}
}
