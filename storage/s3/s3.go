package s3

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
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
	// Endpoint is the host[:port] of the S3 API, without scheme
	// (for example "s3.amazonaws.com" or "localhost:9000").
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	// AccessKeyID, SecretAccessKey, and the optional session Token are static
	// V4 credentials.
	AccessKeyID     string `json:"access_key" yaml:"access_key"`
	SecretAccessKey string `json:"secret" yaml:"secret"`
	Token           string `json:"token" yaml:"token"`
	// Bucket is the bucket every key lives in. It must already exist.
	Bucket string `json:"bucket" yaml:"bucket"`
	// UseSSL selects https for the API and for the derived PublicBase.
	UseSSL bool `json:"use_ssl" yaml:"use_ssl"`
	// PublicBase is the base URL for GenerateURL. Empty means
	// "http(s)://Endpoint/Bucket" (path-style).
	PublicBase string `json:"public_base" yaml:"public_base"`
	// Dir is the prefix directory for keys created by GenerateUploadAuth.
	// It does not affect UploadFile or other key-based methods.
	Dir string `json:"dir" yaml:"dir"`
	// UploadNaming selects how GenerateUploadAuth names files; empty means
	// storage.UploadNamingStrategyRandomExt.
	UploadNaming storage.UploadNamingStrategy `json:"upload_naming" yaml:"upload_naming"`
	// UploadTTL is the default validity window for presigned upload URLs, and
	// also the ceiling for UploadAuthRequest.TTL. A zero value falls back to
	// defaultUploadTTL.
	UploadTTL time.Duration `json:"upload_ttl" yaml:"upload_ttl"`
	// PartSize is the multipart part size for UploadFile, whose reader has an
	// unknown length. minio-go buffers one part in memory per upload, and
	// without a part size it sizes parts for a 5 TiB object (~537 MiB each).
	// The largest object UploadFile accepts is PartSize * 10000. A zero value
	// falls back to defaultPartSize; the minimum is 5 MiB.
	PartSize uint64 `json:"part_size" yaml:"part_size"`
}

const (
	// defaultUploadTTL is the presigned upload URL validity used when neither
	// the request nor the config specifies one.
	defaultUploadTTL = time.Hour
	// defaultPartSize caps UploadFile's per-upload buffer at 16 MiB, allowing
	// objects up to ~156 GiB.
	defaultPartSize = 16 << 20
	// minPartSize is the smallest part S3 accepts.
	minPartSize = 5 << 20
)

// Client is a minio-go storage.CDNStorage: presigned PUT upload auth and
// the core Storage operations. It also implements storage.FileStater and
// storage.FileLister. Create it with NewClient; it is safe for concurrent use
// to the extent minio-go's client is, and has no Close.
type Client struct {
	urlhandler.Handler
	config Config
	client *minio.Client
}

// NewClient creates a minio-backed storage client. An empty PublicBase is
// derived from endpoint+bucket. The minio client is constructed here; this
// type has no Close. NewClient does not contact the server, so bad
// credentials or a missing bucket surface on the first operation. It returns
// an error for a PartSize below 5 MiB, an invalid Endpoint, or an unparsable
// PublicBase.
func NewClient(conf Config) (*Client, error) {
	conf.PartSize = cmp.Or(conf.PartSize, defaultPartSize)
	if conf.PartSize < minPartSize {
		return nil, fmt.Errorf("s3: part size %d is below the 5 MiB minimum", conf.PartSize)
	}
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
	key, err = storage.NormalizeKey(key)
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
// The length is unknown, so the body is sent as a multipart upload buffering
// one Config.PartSize part in memory. Content-Type comes from the key's
// extension. It returns the normalized key.
func (s *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return "", err
	}
	info, err := s.client.PutObject(ctx, s.config.Bucket, key, file, -1, minio.PutObjectOptions{
		ContentType: mime.TypeByExtension(filepath.Ext(key)),
		PartSize:    s.config.PartSize,
	})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

// UploadLocalFile uploads an existing local file to S3-compatible storage with the specified key.
// Content-Type comes from the key's extension. It returns the normalized key.
func (s *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return "", err
	}
	info, err := s.client.FPutObject(ctx, s.config.Bucket, key, file, minio.PutObjectOptions{
		ContentType: mime.TypeByExtension(filepath.Ext(key)),
	})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

// StatFile returns lightweight metadata for a file without downloading its body.
// It implements storage.FileStater by reusing the S3 stat (HEAD) call.
// A missing key fails with storageerr.ErrNotFound.
func (s *Client) StatFile(ctx context.Context, key string) (storage.FileInfo, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return storage.FileInfo{}, err
	}
	info, err := s.client.StatObject(ctx, s.config.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return storage.FileInfo{}, storageerr.ErrNotFound
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
// of the page when more objects remain, otherwise empty. A non-positive limit
// means 1000.
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
// A missing key reports (false, nil).
func (s *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key, err := storage.NormalizeKey(key)
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

// DownloadFile retrieves a file from S3-compatible storage with a single GET.
// Returns the body reader (the caller must close it), the stored content type,
// and the content size. A missing key fails with storageerr.ErrNotFound.
func (s *Client) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return storage.DownloadResult{}, err
	}
	// A single GET supplies both the body and its metadata. minio's lazy
	// Object would HEAD first and then fetch the body with If-Match on the
	// HEAD's ETag, so an overwrite in between failed the read mid-stream with
	// 412 after Size and MIME had already been reported for the old version.
	body, info, _, err := minio.Core{Client: s.client}.GetObject(ctx, s.config.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return storage.DownloadResult{}, storageerr.ErrNotFound
		}
		return storage.DownloadResult{}, err
	}
	return storage.DownloadResult{
		Reader: body,
		MIME:   info.ContentType,
		Size:   info.Size,
	}, nil
}

// DeleteFile removes a file from the S3-compatible storage bucket.
// S3 treats deleting a missing key as success.
func (s *Client) DeleteFile(ctx context.Context, key string) error {
	key, err := storage.NormalizeKey(key)
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
// It performs a copy operation followed by deletion of the source file, so it
// is not atomic: if the delete fails, the object exists at both keys. A move
// onto itself is a no-op when the source exists. Errors follow CopyFile.
func (s *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey, err := storage.NormalizeKey(sourceKey)
	if err != nil {
		return err
	}
	destinationKey, err = storage.NormalizeKey(destinationKey)
	if err != nil {
		return err
	}
	// A move onto itself is a no-op, and must not run copy-then-delete: the
	// delete would remove the very object the copy just wrote, reporting
	// success while destroying the object.
	if sourceKey == destinationKey {
		exists, err := s.IsFileExists(ctx, sourceKey)
		if err != nil {
			return err
		}
		if !exists {
			return storageerr.ErrNotFound
		}
		return nil
	}
	err = s.CopyFile(ctx, sourceKey, destinationKey, overwrite)
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
// A missing source fails with storageerr.ErrNotFound and an existing
// destination with overwrite false fails with storageerr.ErrDestExists.
//
// When overwrite is false the destination is checked with a separate stat call
// before copying. The S3 CopyObject API has no portable conditional (there is
// no If-None-Match on the copy destination), so this guard is best-effort only
// and is NOT a concurrency-safe guarantee: a racing writer between the stat and
// the copy can still be clobbered. MoveFile inherits the same caveat.
func (s *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey, err := storage.NormalizeKey(sourceKey)
	if err != nil {
		return err
	}
	destinationKey, err = storage.NormalizeKey(destinationKey)
	if err != nil {
		return err
	}
	if !overwrite {
		_, err := s.client.StatObject(ctx, s.config.Bucket, destinationKey, minio.StatObjectOptions{})
		if err == nil {
			return storageerr.ErrDestExists
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
			return storageerr.ErrNotFound
		}
		return err
	}
	return nil
}

func isNoSuchKeyError(err error) bool {
	return minio.ToErrorResponse(err).Code == minio.NoSuchKey
}
