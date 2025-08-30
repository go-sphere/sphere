package s3

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"path"
	"strings"
	"time"

	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/go-sphere/sphere/storage/urlhandler"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds the configuration parameters for S3-compatible object storage.
type Config struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key"`
	SecretAccessKey string `json:"secret"`
	Token           string `json:"token"`
	Bucket          string `json:"bucket"`
	UseSSL          bool   `json:"use_ssl"`
	PublicBase      string `json:"public_base"`
}

// Client provides S3-compatible object storage operations with URL handling capabilities.
// It uses the MinIO client library to interact with S3 or S3-compatible services.
type Client struct {
	urlhandler.Handler
	config *Config
	client *minio.Client
}

// NewClient creates a new S3-compatible storage client with the provided configuration.
// It automatically configures the public base URL if not provided and initializes
// the URL handler for public file access.
func NewClient(config *Config) (*Client, error) {
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, config.Token),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	if config.PublicBase == "" {
		if config.UseSSL {
			config.PublicBase = "https://" + config.Endpoint + "/" + config.Bucket
		} else {
			config.PublicBase = "http://" + config.Endpoint + "/" + config.Bucket
		}
	}
	handler, err := urlhandler.NewHandler(config.PublicBase)
	if err != nil {
		return nil, err
	}
	return &Client{
		Handler: *handler,
		config:  config,
		client:  client,
	}, nil
}

// keyPreprocess removes leading slash from storage keys to ensure S3 API compatibility.
func (s *Client) keyPreprocess(key string) string {
	return strings.TrimPrefix(key, "/")
}

// GenerateUploadToken creates a presigned PUT URL for direct client uploads to S3.
// It generates a unique key based on the filename hash and returns the presigned URL,
// storage key, and public access URL. The presigned URL expires after 1 hour.
func (s *Client) GenerateUploadToken(ctx context.Context, fileName string, dir string, nameBuilder func(filename string, dir ...string) string) ([3]string, error) {
	fileExt := path.Ext(fileName)
	sum := md5.Sum([]byte(fileName))
	nameMd5 := hex.EncodeToString(sum[:])
	key := nameBuilder(nameMd5+fileExt, dir)
	key = s.keyPreprocess(key)

	preSignedURL, err := s.client.PresignedPutObject(ctx,
		s.config.Bucket,
		key,
		time.Hour)
	if err != nil {
		return [3]string{}, err
	}
	return [3]string{
		preSignedURL.String(),
		key,
		s.GenerateURL(key),
	}, nil
}

// UploadFile uploads data from a reader to S3-compatible storage with the specified key.
func (s *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	key = s.keyPreprocess(key)
	info, err := s.client.PutObject(ctx, s.config.Bucket, key, file, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

// UploadLocalFile uploads an existing local file to S3-compatible storage with the specified key.
func (s *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	key = s.keyPreprocess(key)
	info, err := s.client.FPutObject(ctx, s.config.Bucket, key, file, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

// IsFileExists checks whether a file exists in the S3-compatible storage bucket.
func (s *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key = s.keyPreprocess(key)
	_, err := s.client.StatObject(ctx, s.config.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DownloadFile retrieves a file from S3-compatible storage.
// Returns the file reader, content type, and content size.
func (s *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	key = s.keyPreprocess(key)
	object, err := s.client.GetObject(ctx, s.config.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", 0, err
	}
	info, err := object.Stat()
	if err != nil {
		return nil, "", 0, err
	}
	return object, info.ContentType, info.Size, nil
}

// DeleteFile removes a file from the S3-compatible storage bucket.
func (s *Client) DeleteFile(ctx context.Context, key string) error {
	key = s.keyPreprocess(key)
	err := s.client.RemoveObject(ctx, s.config.Bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

// MoveFile relocates a file from source to destination key within the S3 bucket.
// It performs a copy operation followed by deletion of the source file.
func (s *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = s.keyPreprocess(sourceKey)
	destinationKey = s.keyPreprocess(destinationKey)
	if !overwrite {
		_, err := s.client.StatObject(ctx, s.config.Bucket, destinationKey, minio.StatObjectOptions{})
		if err == nil {
			return storageerr.ErrorDistExisted
		}
	}
	_, err := s.client.CopyObject(ctx, minio.CopyDestOptions{
		Bucket: s.config.Bucket,
		Object: destinationKey,
	}, minio.CopySrcOptions{
		Bucket: s.config.Bucket,
		Object: sourceKey,
	})
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
func (s *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = s.keyPreprocess(sourceKey)
	destinationKey = s.keyPreprocess(destinationKey)
	if !overwrite {
		_, err := s.client.StatObject(ctx, s.config.Bucket, destinationKey, minio.StatObjectOptions{})
		if err == nil {
			return storageerr.ErrorDistExisted
		}
	}
	_, err := s.client.CopyObject(ctx, minio.CopyDestOptions{
		Bucket: s.config.Bucket,
		Object: destinationKey,
	}, minio.CopySrcOptions{
		Bucket: s.config.Bucket,
		Object: sourceKey,
	})
	if err != nil {
		return err
	}
	return nil
}
