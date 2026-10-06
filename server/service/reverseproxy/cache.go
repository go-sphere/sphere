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
	"sync"

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
//
// The proxy calls Save from a background goroutine while other requests call
// Load, so implementations must be safe for concurrent use. Save should read
// reader until it reports EOF or an error; a reader error means the body is
// incomplete and the entry must not be stored. ServeCacheReverseProxy relies on
// errors.Is(err, ErrCacheNotFound) to tell a miss from a failure.
type Cache interface {
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
	Save(ctx context.Context, key string, header http.Header, reader io.Reader) error
	Load(ctx context.Context, key string) (http.Header, io.ReadCloser, error)
	Header(ctx context.Context, key string) (http.Header, error)
}

// CommonCache stores response headers in a ByteCache and bodies in Storage.
// Each entry is a JSON header blob under the cache key plus a body object named
// by the SHA-256 of the key. Construct it with NewByteCache. Its only locking
// keeps a miss's body cleanup (see NewByteCache) off a key this CommonCache is
// saving; otherwise it is as safe for concurrent use as its backends. The
// guard is per process: instances sharing backends can still clean up a body
// another instance is saving, which leaves that entry unreadable until it is
// saved again. It never closes its backends.
type CommonCache struct {
	cache           cache.ByteCache
	storage         storage.Storage
	setCacheOptions []cache.Option

	mu       sync.Mutex
	keyLocks map[string]*keyLock // per-key Save/cleanup guards in use
}

// keyLock orders Save against orphan cleanup for one key: Save holds it
// shared, cleanup exclusively. refs counts holders so the map entry is
// dropped when the key is idle.
type keyLock struct {
	rw   sync.RWMutex
	refs int
}

func (c *CommonCache) acquireKeyLock(key string) *keyLock {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keyLocks == nil {
		c.keyLocks = make(map[string]*keyLock)
	}
	l := c.keyLocks[key]
	if l == nil {
		l = &keyLock{}
		c.keyLocks[key] = l
	}
	l.refs++
	return l
}

func (c *CommonCache) releaseKeyLock(key string, l *keyLock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	l.refs--
	if l.refs == 0 {
		delete(c.keyLocks, key)
	}
}

// NewByteCache returns a CommonCache. setCacheOptions apply when saving header blobs.
// Use them (for example cache.WithExpiration) to bound how long an entry is
// replayed. Storage has no expiry of its own, so a body object outlives its
// expired header blob until the key is next looked up: a miss on key (Exists,
// Load, Header, or Delete) deletes the body object best-effort unless a Save
// of key is in progress, and a later Save overwrites it. A body whose key is
// never looked up or saved again stays until the storage removes it.
// The caller keeps ownership of cache and storage.
func NewByteCache(cache cache.ByteCache, storage storage.Storage, setCacheOptions ...cache.Option) *CommonCache {
	return &CommonCache{
		cache:           cache,
		storage:         storage,
		setCacheOptions: setCacheOptions,
	}
}

// Exists reports whether key has stored headers and its body object exists.
// A missing header blob, or one without a body pointer, returns an error
// matching ErrCacheNotFound; a missing body object returns false, nil.
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

// Delete removes the header blob for key and, when it names one, the stored
// body object. It returns ErrCacheNotFound when key has no header blob, after
// deleting any body object left behind by an expired blob. A corrupt blob is
// still removed; errors from both deletions are joined.
func (c *CommonCache) Delete(ctx context.Context, key string) error {
	headerRaw, found, err := c.cache.Get(ctx, key)
	if err != nil {
		return err
	}
	if !found {
		c.deleteOrphanBody(ctx, key)
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

// Save uploads reader as the body object for key, then stores a copy of header
// (plus the internal body pointer) under key. header is not modified. If the
// header blob cannot be stored, the uploaded body is deleted on a best-effort
// basis and the error is returned.
func (c *CommonCache) Save(ctx context.Context, key string, header http.Header, reader io.Reader) error {
	l := c.acquireKeyLock(key)
	defer c.releaseKeyLock(key, l)
	l.rw.RLock()
	defer l.rw.RUnlock()
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

// Load returns the stored headers (without the internal body pointer) and a
// reader for the body; the caller must close the reader. A missing header blob
// or body pointer returns an error matching ErrCacheNotFound; a missing body
// object returns the storage's error.
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

// header returns the stored header blob as-is, internal entries included. A
// miss also deletes the body object an expired blob may have left behind.
func (c *CommonCache) header(ctx context.Context, key string) (http.Header, error) {
	headerRaw, found, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if !found {
		c.deleteOrphanBody(ctx, key)
		return nil, ErrCacheNotFound
	}
	header := http.Header{}
	err = json.Unmarshal(headerRaw, &header)
	if err != nil {
		return nil, err
	}
	return header, nil
}

// deleteOrphanBody removes the body object Save would have stored for key. It
// is called when key has no header blob: the header cache expires entries on
// its own but storage does not, so without this the body of every expired
// entry stays in storage for good. Save names the object after the key alone,
// so it can be found without the blob. Errors, including a missing object, are
// ignored: this is cleanup on a path that already reports a miss.
//
// A Save of key in progress has uploaded, or is about to upload, the body it
// will publish, so cleanup is skipped rather than waited for; and because a
// Save may have completed since the caller's miss, the header blob is checked
// again under the lock before deleting.
func (c *CommonCache) deleteOrphanBody(ctx context.Context, key string) {
	l := c.acquireKeyLock(key)
	defer c.releaseKeyLock(key, l)
	if !l.rw.TryLock() {
		return
	}
	defer l.rw.Unlock()
	if _, found, err := c.cache.Get(ctx, key); err != nil || found {
		return
	}
	_ = c.storage.DeleteFile(ctx, storageObjectName(key))
}
