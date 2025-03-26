package kvcache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"

	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/storage/urlhandler"
)

//var _ storage.Storage = (*Client)(nil)

var (
	ErrorNotFound    = errors.New("key not found")
	ErrorDistExisted = errors.New("destination key existed")
)

type Config struct {
	PublicBase string `json:"public_base" yaml:"public_base"`
}

type Client struct {
	*urlhandler.Handler
	config *Config
	cache  cache.ByteCache
}

func NewClient(config *Config, cache cache.ByteCache) (*Client, error) {
	handler, err := urlhandler.NewHandler(config.PublicBase)
	if err != nil {
		return nil, err
	}
	return &Client{
		Handler: handler,
		config:  config,
		cache:   cache,
	}, nil
}

func (c *Client) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (string, error) {
	all, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	err = c.cache.Set(ctx, key, all, -1)
	if err != nil {
		return "", err
	}
	return key, nil
}

func (c *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	err = c.cache.Set(ctx, key, raw, -1)
	if err != nil {
		return "", err
	}
	return key, nil
}

func (c *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	data, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, "", 0, err
	}
	if data == nil {
		return nil, "", 0, err
	}
	return io.NopCloser(bytes.NewReader(*data)), mime.TypeByExtension(filepath.Ext(key)), int64(len(*data)), nil
}

func (c *Client) DeleteFile(ctx context.Context, key string) error {
	err := c.cache.Del(ctx, key)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
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
	if !overwrite {
		value, err := c.cache.Get(ctx, destinationKey)
		if err == nil && value != nil {
			return ErrorDistExisted
		}
	}
	value, err := c.cache.Get(ctx, sourceKey)
	if err != nil {
		return err
	}
	if value == nil {
		return ErrorNotFound
	}
	err = c.cache.Set(ctx, destinationKey, *value, -1)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GenerateUploadToken(fileName string, dir string, nameBuilder func(filename string, dir ...string) string) ([3]string, error) {
	key := nameBuilder(fileName, dir)
	return [3]string{
		"",
		key,
		c.GenerateURL(key),
	}, nil
}

func (c *Client) GenerateImageURL(key string, width int) string {
	log.Warnf("Client not support image resize")
	return c.GenerateURL(key)
}
