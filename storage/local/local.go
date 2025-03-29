package local

import (
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/storage/urlhandler"
)

// var _ storage.Storage = (*Client)(nil)
var (
	ErrorNotFound    = errors.New("file not found")
	ErrorDistExisted = errors.New("destination file existed")
)

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

func (c *Client) filePath(key string) string {
	key = filepath.Clean(key)
	filePath := filepath.Join(c.config.RootDir, key)
	return filePath
}

func (c *Client) logErrorIfPresent(err error) {
	if err != nil {
		log.Errorf("local storage error: %v", err)
	}
}

func (c *Client) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (string, error) {
	filePath := c.filePath(key)
	err := os.MkdirAll(filepath.Dir(filePath), 0o750)
	if err != nil {
		return "", err
	}
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer c.logErrorIfPresent(out.Close())
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
	defer c.logErrorIfPresent(raw.Close())
	return c.UploadFile(ctx, raw, 0, key)
}

func (c *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	filePath := c.filePath(key)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", 0, err
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, "", 0, err
	}
	return file, mime.TypeByExtension(filepath.Ext(key)), stat.Size(), nil
}

func (c *Client) DeleteFile(ctx context.Context, key string) error {
	filePath := c.filePath(key)
	err := os.Remove(filePath)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) removeBeforeOverwrite(path string, overwrite bool) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !overwrite {
		return ErrorDistExisted
	}
	err = os.Remove(path)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourcePath := c.filePath(sourceKey)
	destinationPath := c.filePath(destinationKey)
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o750); err != nil {
		return err
	}
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return ErrorNotFound
	}
	if err := c.removeBeforeOverwrite(destinationPath, overwrite); err != nil {
		return err
	}
	if err := os.Rename(sourcePath, destinationPath); err != nil {
		return err
	}
	return nil
}

func (c *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourcePath := c.filePath(sourceKey)
	destinationPath := c.filePath(destinationKey)
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o750); err != nil {
		return err
	}
	if err := c.removeBeforeOverwrite(destinationPath, overwrite); err != nil {
		return err
	}
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return ErrorNotFound
	}
	defer c.logErrorIfPresent(srcFile.Close())
	dstFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer c.logErrorIfPresent(dstFile.Close())
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	err = dstFile.Sync()
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
