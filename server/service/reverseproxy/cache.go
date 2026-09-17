package reverseproxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/storage"
)

const cacheFileKeyForReverseProxyBody = "X-Cache-ReverseProxy-Body"

// ErrCacheNotFound is returned when no cache entry exists for the requested key.
// It marks an expected cache miss, distinguishing it from an actual cache-layer failure.
var ErrCacheNotFound = errors.New("no cache found")

// Cache persists reverse-proxy response headers and bodies.
// Save stores both; Load returns headers and a body reader; Header returns
// headers only, with the internal body-pointer entry already removed.
//
// Exists and Load report ErrCacheNotFound for a missing header or body pointer.
// Storage errors are returned unchanged.
type Cache interface {
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
	Save(ctx context.Context, key string, header http.Header, reader io.Reader) error
	Load(ctx context.Context, key string) (http.Header, io.ReadCloser, error)
	Header(ctx context.Context, key string) (http.Header, error)
}

// CommonCache stores response headers in a ByteCache and bodies in Storage.
type CommonCache struct {
	cache           cache.ByteCache
	storage         storage.Storage
	setCacheOptions []cache.Option
}

// NewByteCache returns a CommonCache. setCacheOptions apply when saving header blobs.
func NewByteCache(cache cache.ByteCache, storage storage.Storage, setCacheOptions ...cache.Option) *CommonCache {
	return &CommonCache{
		cache:           cache,
		storage:         storage,
		setCacheOptions: setCacheOptions,
	}
}

func (c *CommonCache) Exists(ctx context.Context, key string) (bool, error) {
	header, err := c.header(ctx, key)
	if err != nil {
		return false, err
	}
	cacheFileKey := header.Get(cacheFileKeyForReverseProxyBody)
	if cacheFileKey == "" {
		// The key exists but its stored headers carry no body pointer, so there
		// is no body to serve: a miss, not a cache-layer failure. Callers key on
		// ErrCacheNotFound to tell the two apart — ServeCacheReverseProxy logs
		// anything else at ERROR level for every request.
		return false, fmt.Errorf("reverseproxy: cache entry %q has no stored body: %w", key, ErrCacheNotFound)
	}
	return c.storage.IsFileExists(ctx, cacheFileKey)
}

func (c *CommonCache) Delete(ctx context.Context, key string) error {
	headerRaw, found, err := c.cache.Get(ctx, key)
	if err != nil {
		return err
	}
	if !found {
		return ErrCacheNotFound
	}
	// Best-effort parse: a corrupt or degenerate header blob must still be
	// removable through the public API — failure to recover the body key only
	// skips the storage delete, never the cache delete.
	var cacheFileKey string
	header := http.Header{}
	if jErr := json.Unmarshal(headerRaw, &header); jErr == nil {
		cacheFileKey = header.Get(cacheFileKeyForReverseProxyBody)
	}
	if cacheFileKey == "" {
		return c.cache.Del(ctx, key)
	}
	return errors.Join(
		c.cache.Del(ctx, key),
		c.storage.DeleteFile(ctx, cacheFileKey),
	)
}

// storageObjectName turns a cache key (often a RequestURI such as "/") into a
// filename every storage driver will accept. NormalizeKey("/") is invalid, so
// the raw URI must never be passed to UploadFile.
func storageObjectName(cacheKey string) string {
	sum := sha256.Sum256([]byte(cacheKey))
	return hex.EncodeToString(sum[:])
}

func (c *CommonCache) Save(ctx context.Context, key string, header http.Header, reader io.Reader) error {
	filename := storageObjectName(key)
	cacheFileKey, err := c.storage.UploadFile(ctx, reader, filename)
	if err != nil {
		return err
	}
	// Clone header to avoid modifying the original
	headerCopy := header.Clone()
	headerCopy.Set(cacheFileKeyForReverseProxyBody, cacheFileKey)
	headerRaw, err := json.Marshal(headerCopy)
	if err != nil {
		// Clean up uploaded file on marshal error
		_ = c.storage.DeleteFile(ctx, cacheFileKey)
		return err
	}
	err = cache.Set(ctx, c.cache, key, headerRaw, c.setCacheOptions...)
	if err != nil {
		// Clean up uploaded file on cache set error
		_ = c.storage.DeleteFile(ctx, cacheFileKey)
		return err
	}
	return nil
}

func (c *CommonCache) Load(ctx context.Context, key string) (http.Header, io.ReadCloser, error) {
	header, err := c.header(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	cacheFileKey := header.Get(cacheFileKeyForReverseProxyBody)
	if cacheFileKey == "" {
		// See Exists: a stored entry without a body pointer is a miss.
		return nil, nil, fmt.Errorf("reverseproxy: cache entry %q has no stored body: %w", key, ErrCacheNotFound)
	}
	header.Del(cacheFileKeyForReverseProxyBody)
	result, err := c.storage.DownloadFile(ctx, cacheFileKey)
	if err != nil {
		return nil, nil, err
	}
	return header, result.Reader, nil
}

// Header returns the stored response headers for key, without the internal
// body-pointer entry. That entry names the storage object holding the body —
// an implementation detail callers replaying these headers to a client must
// not publish — so it is stripped here rather than left to the caller.
func (c *CommonCache) Header(ctx context.Context, key string) (http.Header, error) {
	header, err := c.header(ctx, key)
	if err != nil {
		return nil, err
	}
	header.Del(cacheFileKeyForReverseProxyBody)
	return header, nil
}

// header returns the stored header blob as-is, internal entries included.
func (c *CommonCache) header(ctx context.Context, key string) (http.Header, error) {
	headerRaw, found, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrCacheNotFound
	}
	header := http.Header{}
	err = json.Unmarshal(headerRaw, &header)
	if err != nil {
		return nil, err
	}
	return header, nil
}
