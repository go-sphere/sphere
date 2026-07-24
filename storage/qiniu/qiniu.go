package qiniu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/go-sphere/sphere/storage/urlhandler"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	qiniuStorage "github.com/qiniu/go-sdk/v7/storage"
)

// Config holds the configuration parameters for Qiniu Cloud Object Storage integration.
type Config struct {
	AccessKey string `json:"access_key" yaml:"access_key"` // Qiniu access key for authentication
	SecretKey string `json:"secret_key" yaml:"secret_key"` // Qiniu secret key for authentication

	Bucket       string                       `json:"bucket" yaml:"bucket"`               // Storage bucket name
	Dir          string                       `json:"dir" yaml:"dir"`                     // Default directory prefix for uploads
	UploadNaming storage.UploadNamingStrategy `json:"upload_naming" yaml:"upload_naming"` // Upload file naming strategy

	// UploadTTL is the default validity window for generated upload tokens.
	// A zero value lets the Qiniu SDK apply its own default (one hour).
	UploadTTL time.Duration `json:"upload_ttl" yaml:"upload_ttl"`
	// MimeLimit restricts the accepted content types for token uploads using
	// Qiniu's mimeLimit syntax (e.g. "image/*;video/*"). An empty value imposes
	// no restriction, matching the S3 driver's default.
	MimeLimit string `json:"mime_limit" yaml:"mime_limit"`

	PublicBase string `json:"public_base" yaml:"public_base"` // Public base URL for file access
}

// Client provides Qiniu Cloud Object Storage operations with URL handling capabilities.
// It implements storage interfaces for file uploads and URL generation.
type Client struct {
	urlhandler.Handler           // Embedded URL handler for public file access
	config             Config    // Qiniu configuration
	mac                *qbox.Mac // Authentication credentials
}

// NewClient creates a new Qiniu storage client with the provided configuration.
// It initializes the URL handler for public file access and sets up authentication.
// Returns an error if the public base URL is invalid.
func NewClient(conf Config) (*Client, error) {
	handler, err := urlhandler.NewHandler(conf.PublicBase)
	if err != nil {
		return nil, err
	}
	mac := qbox.NewMac(conf.AccessKey, conf.SecretKey)
	return &Client{
		Handler: *handler,
		config:  conf,
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

// GenerateUploadAuth creates a secure upload token for direct client uploads to Qiniu.
// It generates the storage key using configured naming strategy and returns token, key, and public URL.
// Token validity is resolved from req.TTL, falling back to Config.UploadTTL and
// then the Qiniu SDK default. Accepted content types follow Config.MimeLimit
// (empty means unrestricted).
func (n *Client) GenerateUploadAuth(_ context.Context, req storage.UploadAuthRequest) (storage.UploadAuthResult, error) {
	fileName, err := storage.BuildUploadFileName(req.FileName, n.config.UploadNaming)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key, err := storage.JoinUploadKey(n.config.Dir, req.Dir, fileName)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key = n.keyPreprocess(key)
	put := &qiniuStorage.PutPolicy{
		Scope:      n.config.Bucket + ":" + key,
		InsertOnly: 1,
		MimeLimit:  n.config.MimeLimit,
	}
	// PutPolicy.Expires is a deadline expressed in seconds; leaving it zero lets
	// the SDK fall back to its own default (one hour).
	if ttl := n.resolveUploadTTL(req.TTL); ttl > 0 {
		put.Expires = uint64(ttl.Seconds())
	}
	return storage.UploadAuthResult{
		Authorization: storage.UploadAuthorization{
			Type:   storage.UploadAuthorizationTypeToken,
			Value:  put.UploadToken(n.mac),
			Method: http.MethodPost,
		},
		File: storage.UploadFileInfo{
			Key: key,
			URL: n.GenerateURL(key),
		},
	}, nil
}

// UploadFile uploads data from a reader to Qiniu Cloud Object Storage with the specified key.
func (n *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	key = n.keyPreprocess(key)
	put := &qiniuStorage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := qiniuStorage.Config{}
	ret := qiniuStorage.PutRet{}
	formUploader := qiniuStorage.NewFormUploader(&cfg)
	err := formUploader.Put(ctx, &ret, upToken, key, file, -1, nil)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}

// UploadLocalFile uploads an existing local file to Qiniu Cloud Object Storage with the specified key.
func (n *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	key = n.keyPreprocess(key)
	put := &qiniuStorage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := qiniuStorage.Config{}
	ret := qiniuStorage.PutRet{}
	formUploader := qiniuStorage.NewFormUploader(&cfg)
	err := formUploader.PutFile(ctx, &ret, upToken, key, file, nil)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}

// resolveUploadTTL selects the upload token validity, preferring the per request
// TTL, then the configured default. A zero result defers to the SDK default.
func (n *Client) resolveUploadTTL(reqTTL time.Duration) time.Duration {
	if reqTTL > 0 {
		return reqTTL
	}
	return n.config.UploadTTL
}

// StatFile returns lightweight metadata for a file without downloading its body.
// It implements storage.FileStater by reusing the Qiniu stat call.
func (n *Client) StatFile(ctx context.Context, key string) (storage.FileInfo, error) {
	key = n.keyPreprocess(key)
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	info, err := manager.Stat(n.config.Bucket, key)
	if err != nil {
		if isNotFoundError(err) {
			return storage.FileInfo{}, storageerr.ErrorNotFound
		}
		return storage.FileInfo{}, err
	}
	return storage.FileInfo{
		MIME: info.MimeType,
		Size: info.Fsize,
	}, nil
}

// ListFiles enumerates object keys under prefix with cursor-based pagination.
// It implements storage.FileLister on top of the Qiniu list API, using the
// marker as the cursor. The returned next cursor is non-empty only when more
// objects remain.
func (n *Client) ListFiles(ctx context.Context, prefix, cursor string, limit int) ([]string, string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	ret, hasNext, err := manager.ListFilesWithContext(ctx, n.config.Bucket,
		qiniuStorage.ListInputOptionsPrefix(n.keyPreprocess(prefix)),
		qiniuStorage.ListInputOptionsMarker(cursor),
		qiniuStorage.ListInputOptionsLimit(limit),
	)
	if err != nil {
		return nil, "", err
	}
	if ret == nil {
		return nil, "", nil
	}
	keys := make([]string, 0, len(ret.Items))
	for _, item := range ret.Items {
		keys = append(keys, item.Key)
	}
	next := ""
	if hasNext {
		next = ret.Marker
	}
	return keys, next, nil
}

// IsFileExists checks whether a file exists in the Qiniu Cloud Object Storage bucket.
func (n *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key = n.keyPreprocess(key)
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
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
func (n *Client) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	key = n.keyPreprocess(key)
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	object, err := manager.Get(n.config.Bucket, key, &qiniuStorage.GetObjectInput{Context: ctx})
	if err != nil {
		if isNotFoundError(err) {
			return storage.DownloadResult{}, storageerr.ErrorNotFound
		}
		return storage.DownloadResult{}, err
	}
	return storage.DownloadResult{
		Reader: object.Body,
		MIME:   object.ContentType,
		Size:   object.ContentLength,
	}, nil
}

// DeleteFile removes a file from the Qiniu Cloud Object Storage bucket.
func (n *Client) DeleteFile(ctx context.Context, key string) error {
	key = n.keyPreprocess(key)
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	err := manager.Delete(n.config.Bucket, key)
	if err != nil {
		if isNotFoundError(err) {
			return storageerr.ErrorNotFound
		}
		return err
	}
	return nil
}

// MoveFile relocates a file from source to destination key within the Qiniu bucket.
func (n *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = n.keyPreprocess(sourceKey)
	destinationKey = n.keyPreprocess(destinationKey)
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
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
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
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
	if errors.Is(err, qiniuStorage.ErrNoSuchFile) {
		return true
	}
	var respErr *qiniuStorage.ErrorInfo
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr != nil && respErr.Code == 612
}

func isDestinationExistsError(err error) bool {
	var respErr *qiniuStorage.ErrorInfo
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr != nil && respErr.Code == 614
}
