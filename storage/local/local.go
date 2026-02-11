package local

import (
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
)

// Config holds the configuration for local file storage operations.
type Config struct {
	RootDir string `json:"root_dir" yaml:"root_dir"`
}

// Client provides local filesystem storage operations.
// It implements the Storage interface for file operations on the local filesystem.
type Client struct {
	config *Config
}

// NewClient creates a new local storage client with the provided configuration.
// It validates the root directory and creates it if it doesn't exist.
// Returns an error if the root directory cannot be created or is invalid.
func NewClient(config *Config) (*Client, error) {
	if config.RootDir == "" {
		return nil, errors.New("root_dir is required")
	}
	err := os.MkdirAll(config.RootDir, 0o750)
	if err != nil {
		return nil, err
	}
	return &Client{
		config: config,
	}, nil
}

// fixFilePath resolves and validates file paths to prevent directory traversal attacks.
// It ensures that all file operations stay within the configured root directory.
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
	rel, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", storageerr.ErrorFileNameInvalid
	}
	return filePath, nil
}

// UploadFile uploads data from a reader to the local filesystem with the specified key.
// It creates the necessary directory structure and writes the file content.
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
	defer func() {
		_ = out.Close()
	}()
	_, err = io.Copy(out, file)
	if err != nil {
		return "", err
	}
	return key, nil
}

// UploadLocalFile uploads an existing local file to the storage with the specified key.
// This is useful for moving files within the local filesystem storage.
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

// IsFileExists checks whether a file exists in the local filesystem storage.
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

// DownloadFile retrieves a file from local filesystem storage.
// Returns the file reader, MIME type based on file extension, and file size.
func (c *Client) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return storage.DownloadResult{}, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.DownloadResult{}, storageerr.ErrorNotFound
		}
		return storage.DownloadResult{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return storage.DownloadResult{}, err
	}
	return storage.DownloadResult{
		Reader: file,
		MIME:   mime.TypeByExtension(filepath.Ext(key)),
		Size:   stat.Size(),
	}, nil
}

// DeleteFile removes a file from the local filesystem storage.
func (c *Client) DeleteFile(ctx context.Context, key string) error {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return err
	}
	err = os.Remove(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return storageerr.ErrorNotFound
		}
		return err
	}
	return nil
}

// removeBeforeOverwrite handles file overwrite logic for move and copy operations.
// It checks if the destination exists and removes it if overwrite is enabled.
func (c *Client) removeBeforeOverwrite(path string, overwrite bool) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !overwrite {
		return storageerr.ErrorDistExisted
	}
	err = os.Remove(path)
	if err != nil {
		return err
	}
	return nil
}

// MoveFile relocates a file from source to destination key within local filesystem storage.
// Creates necessary directory structure and handles overwrite logic.
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
	if _, e := os.Stat(sourcePath); e != nil {
		if os.IsNotExist(e) {
			return storageerr.ErrorNotFound
		}
		return e
	}
	if e := c.removeBeforeOverwrite(destinationPath, overwrite); e != nil {
		return e
	}
	if e := os.Rename(sourcePath, destinationPath); e != nil {
		return e
	}
	return nil
}

// CopyFile duplicates a file from source to destination key within local filesystem storage.
// Creates necessary directory structure and handles overwrite logic.
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
		if os.IsNotExist(err) {
			return storageerr.ErrorNotFound
		}
		return err
	}
	defer func() {
		_ = srcFile.Close()
	}()
	dstFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = dstFile.Close()
	}()
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
