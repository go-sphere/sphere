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

	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/storage/storageerr"
	"github.com/TBXark/sphere/storage/urlhandler"
)

type Config struct {
	Expires    time.Duration `json:"expires" yaml:"expires"`
	PublicBase string        `json:"public_base" yaml:"public_base"`
}

type Client struct {
	urlhandler.Handler
	config *Config
	cache  cache.ByteCache
}

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

func (c *Client) keyPreprocess(key string) string {
	return strings.TrimPrefix(key, "/")
}

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

func (c *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key = c.keyPreprocess(key)
	_, found, err := c.cache.Get(ctx, key)
	return found, err
}

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

func (c *Client) DeleteFile(ctx context.Context, key string) error {
	key = c.keyPreprocess(key)
	err := c.cache.Del(ctx, key)
	if err != nil {
		return err
	}
	return nil
}

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
