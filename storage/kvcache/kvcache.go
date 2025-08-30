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
	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/go-sphere/sphere/storage/urlhandler"
)

// Config holds the configuration for cache-based storage operations.
type Config struct {
	Expires    time.Duration `json:"expires" yaml:"expires"`
	PublicBase string        `json:"public_base" yaml:"public_base"`
}

// Client provides cache-based storage operations where files are stored in a byte cache.
// This is useful for temporary storage or small files that benefit from fast cache access.
type Client struct {
	urlhandler.Handler
	config *Config
	cache  cache.ByteCache
}

// NewClient creates a new cache-based storage client with the provided configuration and cache backend.
// If no expiration time is specified, files are cached indefinitely.
func NewClient(config *Config, cache cache.ByteCache) (*Client, error) {
	handler, err := urlhandler.NewHandler(config.PublicBase)
	if err != nil {
		return nil, err
	}
	if config.Expires == 0 {
		config.Expires = -1
	}
	return &Client{
		Handler: *handler,
		config:  config,
		cache:   cache,
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
	err = c.cache.SetWithTTL(ctx, key, all, c.config.Expires)
	if err != nil {
		return "", err
	}
	return key, nil
}

// UploadLocalFile reads a local file and stores it in the cache with the specified key.
func (c *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	key = c.keyPreprocess(key)
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	err = c.cache.SetWithTTL(ctx, key, raw, c.config.Expires)
	if err != nil {
		return "", err
	}
	return key, nil
}

// IsFileExists checks whether a file exists in the cache storage.
func (c *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key = c.keyPreprocess(key)
	_, found, err := c.cache.Get(ctx, key)
	return found, err
}

// DownloadFile retrieves file data from the cache storage.
// Returns the file content reader, MIME type based on file extension, and content size.
func (c *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	key = c.keyPreprocess(key)
	data, found, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, "", 0, err
	}
	if !found {
		return nil, "", 0, err
	}
	return io.NopCloser(bytes.NewReader(data)), mime.TypeByExtension(filepath.Ext(key)), int64(len(data)), nil
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
		if !found {
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
	err = c.cache.SetWithTTL(ctx, destinationKey, value, c.config.Expires)
	if err != nil {
		return err
	}
	return nil
}
