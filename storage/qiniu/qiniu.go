package qiniu

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/go-sphere/sphere/storage/urlhandler"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

// Config holds the configuration parameters for Qiniu Cloud Object Storage integration.
type Config struct {
	AccessKey string `json:"access_key" yaml:"access_key"` // Qiniu access key for authentication
	SecretKey string `json:"secret_key" yaml:"secret_key"` // Qiniu secret key for authentication

	Bucket string `json:"bucket" yaml:"bucket"` // Storage bucket name
	Dir    string `json:"dir" yaml:"dir"`       // Default directory prefix for uploads

	PublicBase string `json:"public_base" yaml:"public_base"` // Public base URL for file access
}

// Client provides Qiniu Cloud Object Storage operations with URL handling capabilities.
// It implements storage interfaces for file uploads and URL generation.
type Client struct {
	urlhandler.Handler           // Embedded URL handler for public file access
	config             *Config   // Qiniu configuration
	mac                *qbox.Mac // Authentication credentials
}

// NewClient creates a new Qiniu storage client with the provided configuration.
// It initializes the URL handler for public file access and sets up authentication.
// Returns an error if the public base URL is invalid.
func NewClient(config *Config) (*Client, error) {
	handler, err := urlhandler.NewHandler(config.PublicBase)
	if err != nil {
		return nil, err
	}
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)
	return &Client{
		Handler: *handler,
		config:  config,
		mac:     mac,
	}, nil
}

// GenerateImageURL creates a URL for accessing an image with the specified width using Qiniu's image processing.
// It appends imageView2 parameters to enable automatic image resizing with quality optimization.
func (n *Client) GenerateImageURL(key string, width int) string {
	uri := n.GenerateURL(key)
	res, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	res.RawQuery = fmt.Sprintf("imageView2/2/w/%d/q/75", width)
	return res.String()
}

// keyPreprocess removes leading slash from storage keys to ensure compatibility with Qiniu API.
func (n *Client) keyPreprocess(key string) string {
	return strings.TrimPrefix(key, "/")
}

// GenerateUploadToken creates a secure upload token for direct client uploads to Qiniu.
// It generates a unique key based on the filename hash and returns the token, key, and public URL.
// The token includes restrictions for image and video MIME types only.
func (n *Client) GenerateUploadToken(ctx context.Context, fileName string, dir string, nameBuilder func(fileName string, dir ...string) string) ([3]string, error) {
	fileExt := path.Ext(fileName)
	sum := md5.Sum([]byte(fileName))
	nameMd5 := hex.EncodeToString(sum[:])
	key := nameBuilder(nameMd5+fileExt, n.config.Dir, dir)
	key = n.keyPreprocess(key)
	put := &storage.PutPolicy{
		Scope:      n.config.Bucket + ":" + key,
		InsertOnly: 1,
		MimeLimit:  "image/*;video/*",
	}
	return [3]string{
		put.UploadToken(n.mac),
		key,
		n.GenerateURL(key),
	}, nil
}

// UploadFile uploads data from a reader to Qiniu Cloud Object Storage with the specified key.
func (n *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	key = n.keyPreprocess(key)
	put := &storage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := storage.Config{}
	ret := storage.PutRet{}
	formUploader := storage.NewFormUploader(&cfg)
	err := formUploader.Put(ctx, &ret, upToken, key, file, -1, nil)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}

// UploadLocalFile uploads an existing local file to Qiniu Cloud Object Storage with the specified key.
func (n *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	key = n.keyPreprocess(key)
	put := &storage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := storage.Config{}
	ret := storage.PutRet{}
	formUploader := storage.NewFormUploader(&cfg)
	err := formUploader.PutFile(ctx, &ret, upToken, key, file, nil)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}

// IsFileExists checks whether a file exists in the Qiniu Cloud Object Storage bucket.
func (n *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key = n.keyPreprocess(key)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	_, err := manager.Stat(n.config.Bucket, key)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DownloadFile retrieves a file from Qiniu Cloud Object Storage.
// Returns the file reader, content type, and content length.
func (n *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	key = n.keyPreprocess(key)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	object, err := manager.Get(n.config.Bucket, key, &storage.GetObjectInput{Context: ctx})
	if err != nil {
		if isNotFoundError(err) {
			return nil, "", 0, storageerr.ErrorNotFound
		}
		return nil, "", 0, err
	}
	return object.Body, object.ContentType, object.ContentLength, nil
}

// DeleteFile removes a file from the Qiniu Cloud Object Storage bucket.
func (n *Client) DeleteFile(ctx context.Context, key string) error {
	key = n.keyPreprocess(key)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	err := manager.Delete(n.config.Bucket, key)
	if err != nil {
		return err
	}
	return nil
}

// MoveFile relocates a file from source to destination key within the Qiniu bucket.
func (n *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = n.keyPreprocess(sourceKey)
	destinationKey = n.keyPreprocess(destinationKey)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	err := manager.Move(n.config.Bucket, sourceKey, n.config.Bucket, destinationKey, overwrite)
	if err != nil {
		if isNotFoundError(err) {
			return storageerr.ErrorNotFound
		}
		if !overwrite && isDestinationExistsError(err) {
			return storageerr.ErrorDistExisted
		}
		return err
	}
	return nil
}

// CopyFile duplicates a file from source to destination key within the Qiniu bucket.
func (n *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = n.keyPreprocess(sourceKey)
	destinationKey = n.keyPreprocess(destinationKey)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	err := manager.Copy(n.config.Bucket, sourceKey, n.config.Bucket, destinationKey, overwrite)
	if err != nil {
		if isNotFoundError(err) {
			return storageerr.ErrorNotFound
		}
		if !overwrite && isDestinationExistsError(err) {
			return storageerr.ErrorDistExisted
		}
		return err
	}
	return nil
}

func isNotFoundError(err error) bool {
	if errors.Is(err, storage.ErrNoSuchFile) {
		return true
	}
	var respErr *storage.ErrorInfo
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr != nil && respErr.Code == 612
}

func isDestinationExistsError(err error) bool {
	var respErr *storage.ErrorInfo
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr != nil && respErr.Code == 614
}
