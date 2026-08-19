package s3

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/go-sphere/sphere/storage/urlhandler"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds the configuration parameters for S3-compatible object storage.
type Config struct {
	Endpoint        string                       `json:"endpoint"`
	AccessKeyID     string                       `json:"access_key"`
	SecretAccessKey string                       `json:"secret"`
	Token           string                       `json:"token"`
	Bucket          string                       `json:"bucket"`
	UseSSL          bool                         `json:"use_ssl"`
	PublicBase      string                       `json:"public_base"`
	Dir             string                       `json:"dir" yaml:"dir"`
	UploadNaming    storage.UploadNamingStrategy `json:"upload_naming" yaml:"upload_naming"`
	// UploadTTL is the default validity window for presigned upload URLs, and
	// also the ceiling for UploadAuthRequest.TTL. A zero value falls back to
	// defaultUploadTTL.
	UploadTTL time.Duration `json:"upload_ttl" yaml:"upload_ttl"`
}

// defaultUploadTTL is the presigned upload URL validity used when neither the
// request nor the config specifies one.
const defaultUploadTTL = time.Hour

// Client provides S3-compatible object storage operations with URL handling capabilities.
// It uses the MinIO client library to interact with S3 or S3-compatible services.
type Client struct {
	urlhandler.Handler
	config Config
	client *minio.Client
}

// NewClient creates a new S3-compatible storage client with the provided configuration.
// It automatically configures the public base URL if not provided and initializes
// the URL handler for public file access.
func NewClient(conf Config) (*Client, error) {
	client, err := minio.New(conf.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(conf.AccessKeyID, conf.SecretAccessKey, conf.Token),
		Secure: conf.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	if conf.PublicBase == "" {
		if conf.UseSSL {
			conf.PublicBase = "https://" + conf.Endpoint + "/" + conf.Bucket
		} else {
			conf.PublicBase = "http://" + conf.Endpoint + "/" + conf.Bucket
		}
	}
	handler, err := urlhandler.NewHandler(conf.PublicBase)
	if err != nil {
		return nil, err
	}
	return &Client{
		Handler: *handler,
		config:  conf,
		client:  client,
	}, nil
}

// keyPreprocess normalizes a caller-supplied key into its canonical form.
// See storage.NormalizeKey for the rules; applying them here is what keeps a
// key addressed on one backend addressing the same object on another.
func (s *Client) keyPreprocess(key string) (string, error) {
	return storage.NormalizeKey(key)
}

// GenerateUploadAuth creates a presigned PUT URL for direct client uploads to S3.
// It generates the storage key using configured naming strategy and returns
// the presigned URL, storage key, and public access URL. The presigned URL
// validity defaults to Config.UploadTTL (else defaultUploadTTL); req.TTL may
// shorten it but never extend it.
func (s *Client) GenerateUploadAuth(ctx context.Context, req storage.UploadAuthRequest) (storage.UploadAuthResult, error) {
	fileName, err := storage.BuildUploadFileName(req.FileName, s.config.UploadNaming)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key, err := storage.JoinUploadKey(s.config.Dir, req.Dir, fileName)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key, err = s.keyPreprocess(key)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}

	preSignedURL, err := s.client.PresignedPutObject(ctx,
		s.config.Bucket,
		key,
		storage.ResolveUploadTTL(req.TTL, s.config.UploadTTL, defaultUploadTTL))
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	return storage.UploadAuthResult{
		Authorization: storage.UploadAuthorization{
			Type:   storage.UploadAuthorizationTypeURL,
			Value:  preSignedURL.String(),
			Method: http.MethodPut,
		},
		File: storage.UploadFileInfo{
			Key: key,
			URL: s.GenerateURL(key),
		},
	}, nil
}

// UploadFile uploads data from a reader to S3-compatible storage with the specified key.
func (s *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	key, err := s.keyPreprocess(key)
	if err != nil {
		return "", err
	}
	info, err := s.client.PutObject(ctx, s.config.Bucket, key, file, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

// UploadLocalFile uploads an existing local file to S3-compatible storage with the specified key.
func (s *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	key, err := s.keyPreprocess(key)
	if err != nil {
		return "", err
	}
	info, err := s.client.FPutObject(ctx, s.config.Bucket, key, file, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

// StatFile returns lightweight metadata for a file without downloading its body.
// It implements storage.FileStater by reusing the S3 stat (HEAD) call.
func (s *Client) StatFile(ctx context.Context, key string) (storage.FileInfo, error) {
	key, err := s.keyPreprocess(key)
	if err != nil {
		return storage.FileInfo{}, err
	}
	info, err := s.client.StatObject(ctx, s.config.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return storage.FileInfo{}, storageerr.ErrorNotFound
		}
		return storage.FileInfo{}, err
	}
	return storage.FileInfo{
		MIME: info.ContentType,
		Size: info.Size,
	}, nil
}

// ListFiles enumerates object keys under prefix with cursor-based pagination.
// It implements storage.FileLister on top of the S3 ListObjects API using
// StartAfter as an exclusive cursor. The returned next cursor is the last key
// of the page when more objects remain, otherwise empty.
func (s *Client) ListFiles(ctx context.Context, prefix, cursor string, limit int) ([]string, string, error) {
	if limit <= 0 {
		limit = 1000
	}
	listCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := s.client.ListObjects(listCtx, s.config.Bucket, minio.ListObjectsOptions{
		// A prefix is not a key: an empty prefix means "list everything", so it
		// is only stripped of a leading separator rather than normalized.
		Prefix:     strings.TrimPrefix(prefix, "/"),
		StartAfter: strings.TrimPrefix(cursor, "/"),
		Recursive:  true,
	})
	keys := make([]string, 0, limit)
	next := ""
	for object := range ch {
		if object.Err != nil {
			return nil, "", object.Err
		}
		if len(keys) >= limit {
			// One object beyond the requested page means more remain; stop the
			// listing early and surface a resume cursor.
			next = keys[len(keys)-1]
			cancel()
			break
		}
		keys = append(keys, object.Key)
	}
	// Drain any buffered entries so the ListObjects goroutine can exit cleanly
	// after an early cancel.
	for range ch {
	}
	return keys, next, nil
}

// IsFileExists checks whether a file exists in the S3-compatible storage bucket.
func (s *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key, err := s.keyPreprocess(key)
	if err != nil {
		return false, err
	}
	_, err = s.client.StatObject(ctx, s.config.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DownloadFile retrieves a file from S3-compatible storage.
// Returns the file reader, content type, and content size.
func (s *Client) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	key, err := s.keyPreprocess(key)
	if err != nil {
		return storage.DownloadResult{}, err
	}
	object, err := s.client.GetObject(ctx, s.config.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return storage.DownloadResult{}, storageerr.ErrorNotFound
		}
		return storage.DownloadResult{}, err
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		if isNoSuchKeyError(err) {
			return storage.DownloadResult{}, storageerr.ErrorNotFound
		}
		return storage.DownloadResult{}, err
	}
	return storage.DownloadResult{
		Reader: object,
		MIME:   info.ContentType,
		Size:   info.Size,
	}, nil
}

// DeleteFile removes a file from the S3-compatible storage bucket.
func (s *Client) DeleteFile(ctx context.Context, key string) error {
	key, err := s.keyPreprocess(key)
	if err != nil {
		return err
	}
	err = s.client.RemoveObject(ctx, s.config.Bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

// MoveFile relocates a file from source to destination key within the S3 bucket.
// It performs a copy operation followed by deletion of the source file.
func (s *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	err := s.CopyFile(ctx, sourceKey, destinationKey, overwrite)
	if err != nil {
		return err
	}
	sourceKey, err = s.keyPreprocess(sourceKey)
	if err != nil {
		return err
	}
	err = s.client.RemoveObject(ctx, s.config.Bucket, sourceKey, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

// CopyFile duplicates a file from source to destination key within the S3 bucket.
//
// When overwrite is false the destination is checked with a separate stat call
// before copying. The S3 CopyObject API has no portable conditional (there is
// no If-None-Match on the copy destination), so this guard is best-effort only
// and is NOT a concurrency-safe guarantee: a racing writer between the stat and
// the copy can still be clobbered. MoveFile inherits the same caveat.
func (s *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey, err := s.keyPreprocess(sourceKey)
	if err != nil {
		return err
	}
	destinationKey, err = s.keyPreprocess(destinationKey)
	if err != nil {
		return err
	}
	if !overwrite {
		_, err := s.client.StatObject(ctx, s.config.Bucket, destinationKey, minio.StatObjectOptions{})
		if err == nil {
			return storageerr.ErrorDistExisted
		}
		if !isNoSuchKeyError(err) {
			return err
		}
	}
	_, err = s.client.CopyObject(ctx, minio.CopyDestOptions{
		Bucket: s.config.Bucket,
		Object: destinationKey,
	}, minio.CopySrcOptions{
		Bucket: s.config.Bucket,
		Object: sourceKey,
	})
	if err != nil {
		if isNoSuchKeyError(err) {
			return storageerr.ErrorNotFound
		}
		return err
	}
	return nil
}

func isNoSuchKeyError(err error) bool {
	return minio.ToErrorResponse(err).Code == minio.NoSuchKey
}
