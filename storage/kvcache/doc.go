// Package kvcache stores blobs in a cache.ByteCache and exposes them as a
// storage.Storage.
//
// Use it for small, short-lived, or test objects when a cache backend
// (cache/memory, cache/redis, cache/badgerdb, ...) is already available.
// [NewClient] wraps the cache; the cache remains owned by the caller.
//
// # Usage
//
//	import (
//		"context"
//		"strings"
//		"time"
//
//		"github.com/go-sphere/sphere/cache/memory"
//		"github.com/go-sphere/sphere/storage/kvcache"
//	)
//
//	byteCache := memory.NewByteCache()
//	defer byteCache.Close()
//	ttl := 10 * time.Minute
//	client, err := kvcache.NewClient(kvcache.Config{Expires: &ttl}, byteCache)
//	if err != nil {
//		return err
//	}
//	key, err := client.UploadFile(ctx, strings.NewReader("hello"), "tmp/hello.txt")
//
// # Behavior
//
//   - Uploads read the whole object into memory.
//   - MIME comes from the key's extension, not from the bytes.
//   - Config.Expires sets a TTL on every write; nil means no expiry. Expired
//     objects read as missing.
//   - Durability is the cache's: an evicting cache (such as cache/memory) may
//     drop objects.
//   - MoveFile is copy then delete and is not atomic; the overwrite check is
//     best-effort against concurrent writers.
//   - Client does not implement storage.FileStater, storage.FileLister,
//     storage.URLHandler, or storage.UploadAuthorizer.
package kvcache
