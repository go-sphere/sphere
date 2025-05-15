package reverseproxy

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/storage"
	"io"
	"net/http"
	"strconv"
	"time"
)

const cacheFileKeyForReverseProxyBody = "X-Cache-ReverseProxy-Body"

type Cache interface {
	Delete(ctx context.Context, key string) error
	Save(ctx context.Context, key string, header http.Header, reader io.Reader) error
	Load(ctx context.Context, key string) (http.Header, io.Reader, error)
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

func (c *CommonCache) Delete(ctx context.Context, key string) error {
	headerRaw, err := c.cache.Get(ctx, key)
	if err != nil {
		return err
	}
	header := http.Header{}
	err = json.Unmarshal(*headerRaw, &header)
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
	size, _ := strconv.Atoi(header.Get("Content-Length"))
	filename := key //base64.URLEncoding.EncodeToString([]byte(key))
	cacheFileKey, err := c.storage.UploadFile(ctx, reader, int64(size), filename)
	if err != nil {
		return err
	}
	header.Set(cacheFileKeyForReverseProxyBody, cacheFileKey)
	headerRaw, err := json.Marshal(header)
	if err != nil {
		return err
	}
	err = c.cache.Set(ctx, key, headerRaw, c.expiration)
	if err != nil {
		return err
	}
	return nil
}

func (c *CommonCache) Load(ctx context.Context, key string) (http.Header, io.Reader, error) {
	headerRaw, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	if headerRaw == nil {
		return nil, nil, errors.New("no cache found")
	}
	header := http.Header{}
	err = json.Unmarshal(*headerRaw, &header)
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
