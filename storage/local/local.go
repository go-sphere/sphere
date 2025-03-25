package local

import (
	"context"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/storage/urlhandler"
	"io"
	"os"
	"path/filepath"
)

var _ storage.Storage = (*Client)(nil)

type Config struct {
	RootDir    string `json:"root_dir" yaml:"root_dir"`
	PublicBase string `json:"public_base" yaml:"public_base"`
}

type Client struct {
	*urlhandler.Handler
	config *Config
}

func NewClient(config *Config) (*Client, error) {
	handler, err := urlhandler.NewHandler(config.PublicBase)
	if err != nil {
		return nil, err
	}
	return &Client{
		Handler: handler,
		config:  config,
	}, nil
}

func (c *Client) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (string, error) {
	key = filepath.Clean(key)
	filePath := filepath.Join(c.config.RootDir, key)
	err := os.MkdirAll(filepath.Dir(filePath), 0750)
	if err != nil {
		return "", err
	}
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		return "", err
	}
	return key, nil
}

func (c *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	raw, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer raw.Close()
	return c.UploadFile(ctx, raw, 0, key)
}

func (c *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, error) {
	key = filepath.Clean(key)
	file, err := os.Open(filepath.Join(c.config.RootDir, key))
	if err != nil {
		return nil, err
	}
	return file, nil
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
