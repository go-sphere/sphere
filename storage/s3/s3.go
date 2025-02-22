package s3

import (
	"context"
	"fmt"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/storage/models"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key"`
	SecretAccessKey string `json:"secret"`
	Token           string `json:"token"`
	Bucket          string `json:"bucket"`
	UseSSL          bool   `json:"use_ssl"`
}

type Client struct {
	config *Config
	client *minio.Client
}

func NewClient(config *Config) (*Client, error) {
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, config.Token),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		config: config,
		client: client,
	}, nil
}

func (s *Client) GenerateURL(key string) string {
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	return fmt.Sprintf("%s/%s/%s", s.config.Endpoint, s.config.Bucket, strings.TrimPrefix(key, "/"))
}

func (s *Client) GenerateURLs(keys []string) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		urls[i] = s.GenerateURL(key)
	}
	return urls
}

func (s *Client) GenerateImageURL(key string, width int) string {
	log.Warnf("Client not support image resize")
	return s.GenerateURL(key)
}

func (s *Client) ExtractKeyFromURL(uri string) string {
	key, _ := s.ExtractKeyFromURLWithMode(uri, true)
	return key
}

func (s *Client) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
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
	if u.Host != s.config.Endpoint {
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

func (s *Client) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*models.FileUploadResult, error) {
	info, err := s.client.PutObject(ctx, s.config.Bucket, key, file, size, minio.PutObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &models.FileUploadResult{
		Key: info.Key,
	}, nil
}

func (s *Client) UploadLocalFile(ctx context.Context, file string, key string) (*models.FileUploadResult, error) {
	info, err := s.client.FPutObject(ctx, s.config.Bucket, key, file, minio.PutObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &models.FileUploadResult{
		Key: info.Key,
	}, nil
}
