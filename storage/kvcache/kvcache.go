package kvcache

import (
	"bytes"
	"context"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
)

// Config holds the configuration for cache-based storage operations.
type Config struct {
	Expires *time.Duration `json:"expires" yaml:"expires"`
}

// Client provides cache-based storage operations where files are stored in a byte cache.
// This is useful for temporary storage or small files that benefit from fast cache access.
type Client struct {
	config Config
	cache  cache.ByteCache
}

// NewClient creates a new cache-based storage client with the provided configuration and cache backend.
// If no expiration time is specified, files are cached indefinitely.
func NewClient(conf Config, cache cache.ByteCache) (*Client, error) {
	return &Client{
		config: conf,
		cache:  cache,
	}, nil
}

// keyPreprocess removes leading slash from storage keys to ensure cache key consistency.
func (c *Client) keyPreprocess(key string) string {
	return strings.TrimPrefix(key, "/")
}

// UploadFile stores file data in the cache with the specified key and expiration time.
func (c *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	key = c.keyPreprocess(key)
	all, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	if c.config.Expires != nil {
		err = c.cache.SetWithTTL(ctx, key, all, *c.config.Expires)
	} else {
		err = c.cache.Set(ctx, key, all)
	}
	if err != nil {
		return "", err
	}
	return key, nil
}

// UploadLocalFile reads a local file and stores it in the cache with the specified key.
func (c *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	raw, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = raw.Close()
	}()
	return c.UploadFile(ctx, raw, key)
}

// IsFileExists checks whether a file exists in the cache storage.
func (c *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key = c.keyPreprocess(key)
	_, found, err := c.cache.Get(ctx, key)
	return found, err
}

// DownloadFile retrieves file data from the cache storage.
// Returns the file content reader, MIME type based on file extension, and content size.
func (c *Client) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	key = c.keyPreprocess(key)
	data, found, err := c.cache.Get(ctx, key)
	if err != nil {
		return storage.DownloadResult{}, err
	}
	if !found {
		return storage.DownloadResult{}, storageerr.ErrorNotFound
	}
	return storage.DownloadResult{
		Reader: io.NopCloser(bytes.NewReader(data)),
		MIME:   mime.TypeByExtension(filepath.Ext(key)),
		Size:   int64(len(data)),
	}, nil
}

// DeleteFile removes a file from the cache storage.
func (c *Client) DeleteFile(ctx context.Context, key string) error {
	key = c.keyPreprocess(key)
	err := c.cache.Del(ctx, key)
	if err != nil {
		return err
	}
	return nil
}

// MoveFile relocates a file from source to destination key within cache storage.
// This operation copies the file content and then deletes the source.
func (c *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = c.keyPreprocess(sourceKey)
	destinationKey = c.keyPreprocess(destinationKey)
	err := c.CopyFile(ctx, sourceKey, destinationKey, overwrite)
	if err != nil {
		return err
	}
	err = c.cache.Del(ctx, sourceKey)
	if err != nil {
		return err
	}
	return nil
}

// CopyFile duplicates a file from source to destination key within cache storage.
// Validates overwrite permissions and handles cache expiration settings.
func (c *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = c.keyPreprocess(sourceKey)
	destinationKey = c.keyPreprocess(destinationKey)
	if !overwrite {
		_, found, err := c.cache.Get(ctx, destinationKey)
		if err != nil {
			return err
		}
		if found {
			return storageerr.ErrorDistExisted
		}
	}
	value, found, err := c.cache.Get(ctx, sourceKey)
	if err != nil {
		return err
	}
	if !found {
		return storageerr.ErrorNotFound
	}
	if c.config.Expires != nil {
		err = c.cache.SetWithTTL(ctx, destinationKey, value, *c.config.Expires)
	} else {
		err = c.cache.Set(ctx, destinationKey, value)
	}
	if err != nil {
		return err
	}
	return nil
}
