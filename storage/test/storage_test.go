package test

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/fileserver"
	"github.com/go-sphere/sphere/storage/kvcache"
	"github.com/go-sphere/sphere/storage/local"
	"github.com/go-sphere/sphere/storage/qiniu"
	"github.com/go-sphere/sphere/storage/s3"
)

var _ storage.CDNStorage = (*s3.Client)(nil)
var _ storage.CDNStorage = (*qiniu.Client)(nil)
var _ storage.CDNStorage = (*fileserver.FileServer)(nil)
var _ storage.Storage = (*local.Client)(nil)
var _ storage.Storage = (*kvcache.Client)(nil)

func TestFileServerGenerateUploadAuthWithMemoryImplementations(t *testing.T) {
	ctx := context.Background()
	tokenCache := memory.NewByteCache()
	t.Cleanup(func() { _ = tokenCache.Close() })
	memStorage := newInMemoryStorage(t)

	server, err := fileserver.NewCDNAdapter(
		fileserver.Config{
			PutBase:      "https://cdn.example.com",
			GetBase:      "https://cdn.example.com",
			UploadNaming: storage.UploadNamingStrategyOriginal,
		},
		tokenCache,
		memStorage,
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}

	result, err := server.GenerateUploadAuth(ctx, storage.UploadAuthRequest{
		FileName: "avatar.png",
		Dir:      "users",
	})
	if err != nil {
		t.Fatalf("GenerateUploadAuth() error = %v", err)
	}

	uploadURI := result.Authorization.Value
	key := result.File.Key
	publicURL := result.File.URL
	if key != "users/avatar.png" {
		t.Fatalf("key = %q, want %q", key, "users/avatar.png")
	}
	if publicURL != "https://cdn.example.com/users/avatar.png" {
		t.Fatalf("publicURL = %q, want %q", publicURL, "https://cdn.example.com/users/avatar.png")
	}

	parsed, err := url.Parse(uploadURI)
	if err != nil {
		t.Fatalf("parse upload URI: %v", err)
	}
	if !strings.HasPrefix(parsed.Path, "/") {
		t.Fatalf("upload path = %q, want prefix %q", parsed.Path, "/")
	}
	token := path.Base(parsed.Path)
	if token == "" || token == "." || token == "/" {
		t.Fatalf("invalid token parsed from %q", uploadURI)
	}

	cachedKey, found, err := tokenCache.Get(ctx, token)
	if err != nil {
		t.Fatalf("cache.Get() error = %v", err)
	}
	if !found {
		t.Fatal("upload token not found in cache")
	}
	if string(cachedKey) != key {
		t.Fatalf("cached key = %q, want %q", string(cachedKey), key)
	}
}

func TestFileServerStoragePassThroughWithInMemoryStorage(t *testing.T) {
	ctx := context.Background()
	tokenCache := memory.NewByteCache()
	t.Cleanup(func() { _ = tokenCache.Close() })
	memStorage := newInMemoryStorage(t)

	server, err := fileserver.NewCDNAdapter(
		fileserver.Config{
			PutBase: "https://cdn.example.com",
			GetBase: "https://cdn.example.com",
		},
		tokenCache,
		memStorage,
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}

	const key = "docs/readme.txt"
	uploadKey, err := server.UploadFile(ctx, bytes.NewBufferString("hello"), key)
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if uploadKey != key {
		t.Fatalf("UploadFile() key = %q, want %q", uploadKey, key)
	}

	exists, err := server.IsFileExists(ctx, key)
	if err != nil {
		t.Fatalf("IsFileExists() error = %v", err)
	}
	if !exists {
		t.Fatal("IsFileExists() = false, want true")
	}

	download, err := server.DownloadFile(ctx, key)
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	defer func() { _ = download.Reader.Close() }()
	content, err := io.ReadAll(download.Reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("download content = %q, want %q", string(content), "hello")
	}
	if download.Size != int64(len(content)) {
		t.Fatalf("size = %d, want %d", download.Size, len(content))
	}
	if download.MIME != "text/plain; charset=utf-8" {
		t.Fatalf("mime = %q, want %q", download.MIME, "text/plain; charset=utf-8")
	}
}

func newInMemoryStorage(t *testing.T) *kvcache.Client {
	t.Helper()
	storeCache := memory.NewByteCache()
	t.Cleanup(func() { _ = storeCache.Close() })
	store, err := kvcache.NewClient(kvcache.Config{}, storeCache)
	if err != nil {
		t.Fatalf("new kvcache client: %v", err)
	}
	return store
}
