package reverseproxy

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
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
