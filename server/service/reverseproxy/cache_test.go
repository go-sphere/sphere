package reverseproxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-sphere/sphere/storage"
)

// TestCommonCache_SaveAndLoad tests the cache save and load operations
func TestCommonCache_SaveAndLoad(t *testing.T) {
	cache := setupTestCache(t)

	ctx := (&http.Request{}).Context()
	header := http.Header{
		"Content-Type": []string{"text/plain"},
		"X-Custom":     []string{"value"},
	}
	body := strings.NewReader("test content")

	// Save to cache
	err := cache.Save(ctx, "test-key", header, body)
	if err != nil {
		t.Fatalf("Failed to save to cache: %v", err)
	}

	// Load from cache
	loadedHeader, loadedBody, err := cache.Load(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to load from cache: %v", err)
	}
	defer func() {
		if closer, ok := loadedBody.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	// Verify header
	if loadedHeader.Get("Content-Type") != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", loadedHeader.Get("Content-Type"))
	}
	if loadedHeader.Get("X-Custom") != "value" {
		t.Errorf("Expected X-Custom 'value', got '%s'", loadedHeader.Get("X-Custom"))
	}

	// Verify body
	loadedContent, err := io.ReadAll(loadedBody)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}
	if string(loadedContent) != "test content" {
		t.Errorf("Expected 'test content', got '%s'", string(loadedContent))
	}
}

func TestCommonCache_SaveRootPath(t *testing.T) {
	cache := setupTestCache(t)
	ctx := t.Context()
	header := http.Header{"Content-Type": []string{"text/plain"}}

	if err := cache.Save(ctx, "/", header, strings.NewReader("homepage")); err != nil {
		t.Fatalf("Save GET /: %v", err)
	}
	loadedHeader, loadedBody, err := cache.Load(ctx, "/")
	if err != nil {
		t.Fatalf("Load GET /: %v", err)
	}
	defer ignoreCloseError(loadedBody.Close)
	if loadedHeader.Get("Content-Type") != "text/plain" {
		t.Errorf("Content-Type = %q", loadedHeader.Get("Content-Type"))
	}
	got, err := io.ReadAll(loadedBody)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "homepage" {
		t.Errorf("body = %q, want homepage", got)
	}

	if err := cache.Save(ctx, "/foo", header, strings.NewReader("without slash")); err != nil {
		t.Fatalf("Save /foo: %v", err)
	}
	if err := cache.Save(ctx, "/foo/", header, strings.NewReader("with slash")); err != nil {
		t.Fatalf("Save /foo/: %v", err)
	}
	_, otherBody, err := cache.Load(ctx, "/foo")
	if err != nil {
		t.Fatalf("Load /foo: %v", err)
	}
	defer ignoreCloseError(otherBody.Close)
	got, err = io.ReadAll(otherBody)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "without slash" {
		t.Errorf("/foo body clobbered by /foo/: %q", got)
	}
	_, slashBody, err := cache.Load(ctx, "/foo/")
	if err != nil {
		t.Fatalf("Load /foo/: %v", err)
	}
	defer ignoreCloseError(slashBody.Close)
	got, err = io.ReadAll(slashBody)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "with slash" {
		t.Errorf("/foo/ body clobbered by /foo: %q", got)
	}
}

// TestCommonCache_Delete tests cache deletion
func TestCommonCache_Delete(t *testing.T) {
	cache := setupTestCache(t)

	ctx := (&http.Request{}).Context()
	header := http.Header{"Content-Type": []string{"text/plain"}}
	body := strings.NewReader("delete test")

	// Save to cache
	err := cache.Save(ctx, "delete-key", header, body)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Delete from cache
	err = cache.Delete(ctx, "delete-key")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	// Verify deletion
	_, _, err = cache.Load(ctx, "delete-key")
	if err == nil {
		t.Error("Expected error when loading deleted key, got nil")
	}
}

// TestCommonCache_Exists tests cache existence check
func TestCommonCache_Exists(t *testing.T) {
	cache := setupTestCache(t)

	ctx := (&http.Request{}).Context()

	// Check non-existent key
	exists, err := cache.Exists(ctx, "non-existent")
	if !errors.Is(err, ErrCacheNotFound) {
		t.Fatalf("Exists(non-existent) error = %v, want %v", err, ErrCacheNotFound)
	}
	if exists {
		t.Fatal("non-existent key exists")
	}

	// Save and check
	header := http.Header{"Content-Type": []string{"text/plain"}}
	body := strings.NewReader("exists test")
	err = cache.Save(ctx, "exists-key", header, body)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	exists, err = cache.Exists(ctx, "exists-key")
	if err != nil {
		t.Fatalf("Exists(saved key): %v", err)
	}
	if !exists {
		t.Fatal("saved key does not exist")
	}
}

// TestCommonCache_ExpiredHeaderDoesNotOrphanBody pins that once a header blob
// is gone (expired or evicted by the header cache), the body object it pointed
// at does not stay in storage forever: the miss reclaims it.
func TestCommonCache_ExpiredHeaderDoesNotOrphanBody(t *testing.T) {
	for _, tc := range []struct {
		name string
		miss func(ctx context.Context, c *CommonCache, key string) error
	}{
		{name: "Load", miss: func(ctx context.Context, c *CommonCache, key string) error {
			_, _, err := c.Load(ctx, key)
			return err
		}},
		{name: "Exists", miss: func(ctx context.Context, c *CommonCache, key string) error {
			_, err := c.Exists(ctx, key)
			return err
		}},
		{name: "Header", miss: func(ctx context.Context, c *CommonCache, key string) error {
			_, err := c.Header(ctx, key)
			return err
		}},
		{name: "Delete", miss: func(ctx context.Context, c *CommonCache, key string) error {
			return c.Delete(ctx, key)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := setupTestCache(t)
			ctx := t.Context()
			const key = "/expiring"

			if err := c.Save(ctx, key, http.Header{}, strings.NewReader("body")); err != nil {
				t.Fatalf("Save: %v", err)
			}
			// Simulate the header cache expiring the entry on its own.
			if err := c.cache.Del(ctx, key); err != nil {
				t.Fatalf("expire header: %v", err)
			}
			if err := tc.miss(ctx, c, key); !errors.Is(err, ErrCacheNotFound) {
				t.Fatalf("%s after expiry = %v, want ErrCacheNotFound", tc.name, err)
			}
			exists, err := c.storage.IsFileExists(ctx, storageObjectName(key))
			if err != nil {
				t.Fatalf("IsFileExists: %v", err)
			}
			if exists {
				t.Fatal("body object outlived its expired header")
			}
		})
	}
}

// uploadHookStorage runs afterUpload once a body upload has completed, i.e.
// inside Save between storing the body and storing its header blob.
type uploadHookStorage struct {
	storage.Storage
	afterUpload func()
}

func (s *uploadHookStorage) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	stored, err := s.Storage.UploadFile(ctx, file, key)
	if err == nil && s.afterUpload != nil {
		s.afterUpload()
	}
	return stored, err
}

// TestCommonCache_MissDuringSaveKeepsBody pins that the orphan cleanup run by
// a miss does not delete the body a concurrent Save of the same key has just
// uploaded but not yet published: the entry must be complete once Save
// returns.
func TestCommonCache_MissDuringSaveKeepsBody(t *testing.T) {
	c := setupTestCache(t)
	ctx := t.Context()
	const key = "/in-flight"

	c.storage = &uploadHookStorage{Storage: c.storage, afterUpload: func() {
		if _, err := c.Exists(ctx, key); !errors.Is(err, ErrCacheNotFound) {
			t.Errorf("Exists during Save = %v, want ErrCacheNotFound", err)
		}
	}}
	if err := c.Save(ctx, key, http.Header{}, strings.NewReader("body")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, body, err := c.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	defer ignoreCloseError(body.Close)
	if got, err := io.ReadAll(body); err != nil || string(got) != "body" {
		t.Fatalf("body = %q, %v; want %q", got, err, "body")
	}
}
