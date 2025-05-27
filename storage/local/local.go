package local

import (
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/TBXark/sphere/storage/urlhandler"
	"github.com/TBXark/sphere/utils/safe"
)

var (
	ErrorNotFound        = errors.New("file not found")
	ErrorDistExisted     = errors.New("destination file existed")
	ErrorFileOutsideRoot = errors.New("file path outside root dir")
	ErrorFileNameInvalid = errors.New("file name invalid")
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
	if config.RootDir == "" {
		return nil, errors.New("root_dir is required")
	}
	err = os.MkdirAll(config.RootDir, 0o750)
	if err != nil {
		return nil, err
	}
	return &Client{
		Handler: handler,
		config:  config,
	}, nil
}

func (c *Client) fixFilePath(key string) (string, error) {
	rootDir, err := filepath.Abs(c.config.RootDir)
	if err != nil {
		return "", err
	}
	filePath, err := filepath.Abs(filepath.Join(c.config.RootDir, filepath.Clean(key)))
	if err != nil {
		return "", err
	}
	rootDir = filepath.Clean(rootDir)
	filePath = filepath.Clean(filePath)
	if !strings.HasPrefix(filePath, rootDir) {
		return "", ErrorFileOutsideRoot
	}
	return filePath, nil
}

func (c *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(filepath.Dir(filePath), 0o750)
	if err != nil {
		return "", err
	}
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer safe.IfErrorPresent("close file", out.Close)
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
	defer safe.IfErrorPresent("close file", raw.Close)
	return c.UploadFile(ctx, raw, key)
}

func (c *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return nil, "", 0, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", 0, ErrorNotFound
		}
		return nil, "", 0, err
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, "", 0, err
	}
	return file, mime.TypeByExtension(filepath.Ext(key)), stat.Size(), nil
}

func (c *Client) DeleteFile(ctx context.Context, key string) error {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return err
	}
	err = os.Remove(filePath)
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
	sourcePath, err := c.fixFilePath(sourceKey)
	if err != nil {
		return err
	}
	destinationPath, err := c.fixFilePath(destinationKey)
	if err != nil {
		return err
	}
	if e := os.MkdirAll(filepath.Dir(destinationPath), 0o750); e != nil {
		return e
	}
	if _, e := os.Stat(sourcePath); os.IsNotExist(e) {
		return ErrorNotFound
	}
	if e := c.removeBeforeOverwrite(destinationPath, overwrite); e != nil {
		return e
	}
	if e := os.Rename(sourcePath, destinationPath); e != nil {
		return e
	}
	return nil
}

func (c *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourcePath, err := c.fixFilePath(sourceKey)
	if err != nil {
		return err
	}
	destinationPath, err := c.fixFilePath(destinationKey)
	if err != nil {
		return err
	}
	if e := os.MkdirAll(filepath.Dir(destinationPath), 0o750); e != nil {
		return e
	}
	if e := c.removeBeforeOverwrite(destinationPath, overwrite); e != nil {
		return e
	}
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return ErrorNotFound
	}
	defer safe.IfErrorPresent("close file", srcFile.Close)
	dstFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer safe.IfErrorPresent("close file", dstFile.Close)
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
