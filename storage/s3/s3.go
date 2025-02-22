package s3

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/TBXark/sphere/log"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

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

func (s *Client) hasHttpScheme(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}

func (s *Client) GenerateURL(key string) string {
	if key == "" {
		return ""
	}
	if s.hasHttpScheme(key) {
		return key
	}
	uri := fmt.Sprintf("%s/%s/%s", s.config.Endpoint, s.config.Bucket, strings.TrimPrefix(key, "/"))
	if s.hasHttpScheme(uri) {
		return uri
	}
	if s.config.UseSSL {
		uri = "https://" + uri
	} else {
		uri = "http://" + uri
	}
	return uri
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
	if !s.hasHttpScheme(uri) {
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
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
	if len(parts) != 2 || parts[0] != s.config.Bucket {
		if strict {
			return "", fmt.Errorf("invalid url")
		}
		return uri, nil
	}
	return parts[1], nil
}

func (s *Client) GenerateUploadToken(fileName string, dir string, nameBuilder func(filename string, dir ...string) string) ([3]string, error) {
	fileExt := path.Ext(fileName)
	sum := md5.Sum([]byte(fileName))
	nameMd5 := hex.EncodeToString(sum[:])
	key := nameBuilder(nameMd5+fileExt, dir)
	key = strings.TrimPrefix(key, "/")

	preSignedURL, err := s.client.PresignedPutObject(context.Background(),
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

func (s *Client) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (string, error) {
	info, err := s.client.PutObject(ctx, s.config.Bucket, key, file, size, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

func (s *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	info, err := s.client.FPutObject(ctx, s.config.Bucket, key, file, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

func (s *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.client.GetObject(ctx, s.config.Bucket, key, minio.GetObjectOptions{})
}
