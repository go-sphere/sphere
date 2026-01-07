package reverseproxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/storage"
)

const cacheFileKeyForReverseProxyBody = "X-Cache-ReverseProxy-Body"

type Cache interface {
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
	Save(ctx context.Context, key string, header http.Header, reader io.Reader) error
	Load(ctx context.Context, key string) (http.Header, io.ReadCloser, error)
	Header(ctx context.Context, key string) (http.Header, error)
}

type CommonCache struct {
	cache           cache.ByteCache
	storage         storage.Storage
	setCacheOptions []cache.Option
}

func NewByteCache(cache cache.ByteCache, storage storage.Storage, setCacheOptions ...cache.Option) *CommonCache {
	return &CommonCache{
		cache:           cache,
		storage:         storage,
		setCacheOptions: setCacheOptions,
	}
}

func (c *CommonCache) Exists(ctx context.Context, key string) (bool, error) {
	header, err := c.Header(ctx, key)
	if err != nil {
		return false, err
	}
	cacheFileKey := header.Get(cacheFileKeyForReverseProxyBody)
	if cacheFileKey == "" {
		return false, errors.New("no cache file found")
	}
	return c.storage.IsFileExists(ctx, cacheFileKey)
}

func (c *CommonCache) Delete(ctx context.Context, key string) error {
	header, err := c.Header(ctx, key)
	if err != nil {
		return err
	}
	cacheFileKey := header.Get(cacheFileKeyForReverseProxyBody)
	if cacheFileKey == "" {
		return nil
	}
	return errors.Join(
		c.cache.Del(ctx, key),
		c.storage.DeleteFile(ctx, cacheFileKey),
	)
}

func (c *CommonCache) Save(ctx context.Context, key string, header http.Header, reader io.Reader) error {
	filename := key // base64 or uuid
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
	header, err := c.Header(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	cacheFileKey := header.Get(cacheFileKeyForReverseProxyBody)
	if cacheFileKey == "" {
		return nil, nil, errors.New("no cache file found")
	}
	header.Del(cacheFileKeyForReverseProxyBody)
	reader, _, _, err := c.storage.DownloadFile(ctx, cacheFileKey)
	if err != nil {
		return nil, nil, err
	}
	return header, reader, nil
}

func (c *CommonCache) Header(ctx context.Context, key string) (http.Header, error) {
	headerRaw, found, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("no cache found")
	}
	header := http.Header{}
	err = json.Unmarshal(headerRaw, &header)
	if err != nil {
		return nil, err
	}
	return header, nil
}
