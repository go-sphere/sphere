package s3

import (
	"context"
	"fmt"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/storage"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	EndPoint        string `json:"end_point"`
	AccessKeyID     string `json:"access_key"`
	SecretAccessKey string `json:"secret"`
	Token           string `json:"token"`
	Bucket          string `json:"bucket"`
	UseSSL          bool   `json:"use_ssl"`
}

type S3 struct {
	config *Config
	client *minio.Client
}

func NewS3(config *Config) (*S3, error) {
	client, err := minio.New(config.EndPoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, config.Token),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &S3{
		config: config,
		client: client,
	}, nil
}

func (s *S3) GenerateURL(key string) string {
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	return fmt.Sprintf("%s/%s/%s", s.config.EndPoint, s.config.Bucket, strings.TrimPrefix(key, "/"))
}

func (s *S3) GenerateURLs(keys []string) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		urls[i] = s.GenerateURL(key)
	}
	return urls
}

func (s *S3) GenerateImageURL(key string, width int) string {
	log.Warnf("S3 not support image resize")
	return s.GenerateURL(key)
}

func (s *S3) ExtractKeyFromURL(uri string) string {
	key, _ := s.ExtractKeyFromURLWithMode(uri, true)
	return key
}

func (s *S3) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	if uri == "" {
		return "", nil
	}
	if !(strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")) {
		return strings.TrimPrefix(uri, "/"), nil
	}
	u, err := url.Parse(uri)
	if err != nil {
		return "", nil
	}
	if u.Host != s.config.EndPoint {
		if strict {
			return "", fmt.Errorf("invalid url")
		}
		return uri, nil
	}
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] != s.config.Bucket {
		if strict {
			return "", fmt.Errorf("invalid url")
		}
		return uri, nil
	}
	return parts[1], nil
}

func (s *S3) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*storage.FileUploadResult, error) {
	info, err := s.client.PutObject(ctx, s.config.Bucket, key, file, size, minio.PutObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &storage.FileUploadResult{
		Key: info.Key,
	}, nil
}

func (s *S3) UploadLocalFile(ctx context.Context, file string, key string) (*storage.FileUploadResult, error) {
	info, err := s.client.FPutObject(ctx, s.config.Bucket, key, file, minio.PutObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &storage.FileUploadResult{
		Key: info.Key,
	}, nil
}
