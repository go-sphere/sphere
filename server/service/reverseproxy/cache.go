package reverseproxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/storage"
)

const cacheFileKeyForReverseProxyBody = "X-Cache-ReverseProxy-Body"

type Cache interface {
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
	Save(ctx context.Context, key string, header http.Header, reader io.Reader) error
	Load(ctx context.Context, key string) (http.Header, io.Reader, error)
	Header(ctx context.Context, key string) (http.Header, error)
}

type CommonCache struct {
	expiration time.Duration
	cache      cache.ByteCache
	storage    storage.Storage
}

func NewByteCache(expiration time.Duration, cache cache.ByteCache, storage storage.Storage) *CommonCache {
	return &CommonCache{
		expiration: expiration,
		cache:      cache,
		storage:    storage,
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
	filename := key // base64.URLEncoding.EncodeToString([]byte(key))
	cacheFileKey, err := c.storage.UploadFile(ctx, reader, filename)
	if err != nil {
		return err
	}
	header.Set(cacheFileKeyForReverseProxyBody, cacheFileKey)
	headerRaw, err := json.Marshal(header)
	if err != nil {
		return err
	}
	err = c.cache.SetWithTTL(ctx, key, headerRaw, c.expiration)
	if err != nil {
		return err
	}
	return nil
}

func (c *CommonCache) Load(ctx context.Context, key string) (http.Header, io.Reader, error) {
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
