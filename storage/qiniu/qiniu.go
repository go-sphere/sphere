// Package qiniu is a Kodo storage.CDNStorage: token upload auth, public
// URLHandler, and server-side upload/download/delete/move/copy.
//
// Upload tokens use InsertOnly: 1. MimeLimit applies to the token path
// (declared Content-Type), not byte sniffing, and not server-side
// UploadFile. PutPolicy.Expires is relative seconds (sub-second rounded up
// to 1). Delete of miss is idempotent. Download Size comes from Stat.
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

	// UploadTTL is the default validity window for generated upload tokens, and
	// also the ceiling for UploadAuthRequest.TTL. A zero value falls back to
	// defaultUploadTTL.
	UploadTTL time.Duration `json:"upload_ttl" yaml:"upload_ttl"`
	// MimeLimit restricts the accepted content types for token uploads using
	// Qiniu's mimeLimit syntax (e.g. "image/*;video/*"). An empty value applies
	// DefaultMimeLimit; set it to AnyMimeLimit to accept every content type.
	MimeLimit string `json:"mime_limit" yaml:"mime_limit"`

	PublicBase string `json:"public_base" yaml:"public_base"` // Public base URL for file access
}

const (
	// DefaultMimeLimit is applied when Config.MimeLimit is empty. Uploads land in
	// a CDN-fronted public bucket under a client-influenced key, so restricting
	// the declared content type is the safer default; opt out with AnyMimeLimit.
	DefaultMimeLimit = "image/*;video/*"
	// AnyMimeLimit disables MIME restrictions for token uploads. Note that Qiniu
	// only validates the Content-Type the client declares, never the bytes it
	// sends, so neither value is a substitute for validating uploads server side.
	AnyMimeLimit = "*/*"
)

// defaultUploadTTL is the upload token validity used when Config.UploadTTL is
// unset. It matches the default the Qiniu SDK would otherwise apply, but is
// stated here so the ceiling for UploadAuthRequest.TTL does not depend on it.
const defaultUploadTTL = time.Hour

// Client is a Kodo storage.CDNStorage: token upload auth, public URLs, and
// server-side upload/download/delete/move/copy.
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
	// Normalize the MIME policy once so GenerateUploadAuth can use it verbatim:
	// empty means "unset" and takes the restrictive default, while AnyMimeLimit
	// is the explicit opt out and maps to the empty PutPolicy field Qiniu reads
	// as "no restriction".
	switch conf.MimeLimit {
	case "":
		conf.MimeLimit = DefaultMimeLimit
	case AnyMimeLimit:
		conf.MimeLimit = ""
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

// GenerateUploadAuth creates a secure upload token for direct client uploads to Qiniu.
// It generates the storage key using configured naming strategy and returns token, key, and public URL.
// Token validity defaults to Config.UploadTTL (else defaultUploadTTL); req.TTL
// may shorten it but never extend it. Accepted content types follow
// Config.MimeLimit, which NewClient normalizes to DefaultMimeLimit when empty.
func (n *Client) GenerateUploadAuth(_ context.Context, req storage.UploadAuthRequest) (storage.UploadAuthResult, error) {
	fileName, err := storage.BuildUploadFileName(req.FileName, n.config.UploadNaming)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key, err := storage.JoinUploadKey(n.config.Dir, req.Dir, fileName)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key, err = storage.NormalizeKey(key)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	put := &qiniuStorage.PutPolicy{
		Scope:      n.config.Bucket + ":" + key,
		InsertOnly: 1,
		MimeLimit:  n.config.MimeLimit,
	}
	// Despite the "deadline" JSON name, PutPolicy.Expires is a *relative* number
	// of seconds: the SDK adds time.Now() to it while signing. Leaving it zero
	// would let the SDK apply its own default, so round sub-second TTLs up to one
	// second rather than truncating them to zero and silently widening the token
	// to an hour.
	seconds := uint64(storage.ResolveUploadTTL(req.TTL, n.config.UploadTTL, defaultUploadTTL).Seconds())
	if seconds == 0 {
		seconds = 1
	}
	put.Expires = seconds
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
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return "", err
	}
	put := &qiniuStorage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := qiniuStorage.Config{}
	ret := qiniuStorage.PutRet{}
	formUploader := qiniuStorage.NewFormUploader(&cfg)
	err = formUploader.Put(ctx, &ret, upToken, key, file, -1, nil)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}

// UploadLocalFile uploads an existing local file to Qiniu Cloud Object Storage with the specified key.
func (n *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return "", err
	}
	put := &qiniuStorage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := qiniuStorage.Config{}
	ret := qiniuStorage.PutRet{}
	formUploader := qiniuStorage.NewFormUploader(&cfg)
	err = formUploader.PutFile(ctx, &ret, upToken, key, file, nil)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}

// StatFile returns lightweight metadata for a file without downloading its body.
// It implements storage.FileStater by reusing the Qiniu stat call.
func (n *Client) StatFile(ctx context.Context, key string) (storage.FileInfo, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return storage.FileInfo{}, err
	}
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
		// A prefix is not a key: an empty prefix means "list everything", so it
		// is only stripped of a leading separator rather than normalized.
		qiniuStorage.ListInputOptionsPrefix(strings.TrimPrefix(prefix, "/")),
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
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return false, err
	}
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	_, err = manager.Stat(n.config.Bucket, key)
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
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return storage.DownloadResult{}, err
	}
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	object, err := manager.Get(n.config.Bucket, key, &qiniuStorage.GetObjectInput{Context: ctx})
	if err != nil {
		if isDownloadNotFoundError(err) {
			return storage.DownloadResult{}, storageerr.ErrorNotFound
		}
		return storage.DownloadResult{}, err
	}
	// The SDK fills GetObjectOutput.ContentLength only after the download
	// goroutine has finished writing the body, while Get returns as soon as the
	// response headers arrive — so the field read here is always zero, and
	// reading it at all races that goroutine. Size is contractually the number
	// of readable bytes and drives Content-Length on the HTTP path, where a zero
	// makes the server send an empty body, so it is resolved with a stat instead.
	// This mirrors the s3 driver, which stats the object for the same reason.
	info, err := manager.Stat(n.config.Bucket, key)
	if err != nil {
		if object.Body != nil {
			_ = object.Body.Close()
		}
		if isDownloadNotFoundError(err) {
			return storage.DownloadResult{}, storageerr.ErrorNotFound
		}
		return storage.DownloadResult{}, err
	}
	mime := object.ContentType
	if mime == "" {
		mime = info.MimeType
	}
	return storage.DownloadResult{
		Reader: object.Body,
		MIME:   mime,
		Size:   info.Fsize,
	}, nil
}

// DeleteFile removes a file from the Qiniu Cloud Object Storage bucket.
// Deletion is idempotent: a key that does not exist reports success.
func (n *Client) DeleteFile(ctx context.Context, key string) error {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return err
	}
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	err = manager.Delete(n.config.Bucket, key)
	if err != nil {
		// Qiniu reports a missing key as 612; the delete contract is idempotent,
		// so treat "already gone" as success.
		if isNotFoundError(err) {
			return nil
		}
		return err
	}
	return nil
}

// MoveFile relocates a file from source to destination key within the Qiniu bucket.
func (n *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey, err := storage.NormalizeKey(sourceKey)
	if err != nil {
		return err
	}
	destinationKey, err = storage.NormalizeKey(destinationKey)
	if err != nil {
		return err
	}
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	err = manager.Move(n.config.Bucket, sourceKey, n.config.Bucket, destinationKey, overwrite)
	if err != nil {
		if isNotFoundError(err) {
			return storageerr.ErrNotFound
		}
		if !overwrite && isDestinationExistsError(err) {
			return storageerr.ErrDestExists
		}
		return err
	}
	return nil
}

// CopyFile duplicates a file from source to destination key within the Qiniu bucket.
func (n *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey, err := storage.NormalizeKey(sourceKey)
	if err != nil {
		return err
	}
	destinationKey, err = storage.NormalizeKey(destinationKey)
	if err != nil {
		return err
	}
	manager := qiniuStorage.NewBucketManager(n.mac, &qiniuStorage.Config{})
	err = manager.Copy(n.config.Bucket, sourceKey, n.config.Bucket, destinationKey, overwrite)
	if err != nil {
		if isNotFoundError(err) {
			return storageerr.ErrNotFound
		}
		if !overwrite && isDestinationExistsError(err) {
			return storageerr.ErrDestExists
		}
		return err
	}
	return nil
}

// isNotFoundError reports a missing key on the bucket-management (rs) API, which
// answers with Qiniu's own 612 status rather than an HTTP 404.
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

// isDownloadNotFoundError reports a missing key on the download path. Downloads
// go through the object source rather than the rs API, so a missing key comes
// back as a plain HTTP 404 that the 612 check above does not recognise; without
// this the caller cannot tell "no such object" from a transport failure, and an
// HTTP handler answers 500 where it should answer 404. The 612 case is still
// accepted because the same helper guards the stat performed alongside the
// download.
func isDownloadNotFoundError(err error) bool {
	if isNotFoundError(err) {
		return true
	}
	var respErr *qiniuStorage.ErrorInfo
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr != nil && respErr.Code == http.StatusNotFound
}

func isDestinationExistsError(err error) bool {
	var respErr *qiniuStorage.ErrorInfo
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr != nil && respErr.Code == 614
}
