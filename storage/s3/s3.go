package s3

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"path"
	"strings"
	"time"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/storage/urlhandler"
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
	PublicBase      string `json:"public_base"`
}

type Client struct {
	*urlhandler.Handler
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
	if config.PublicBase == "" {
		if config.UseSSL {
			config.PublicBase = "https://" + config.Endpoint + "/" + config.Bucket
		} else {
			config.PublicBase = "http://" + config.PublicBase + "/" + config.Bucket
		}
	}
	handler, err := urlhandler.NewHandler(config.PublicBase)
	if err != nil {
		return nil, err
	}
	return &Client{
		Handler: handler,
		config:  config,
		client:  client,
	}, nil
}

func (s *Client) GenerateImageURL(key string, width int) string {
	log.Warnf("Client not support image resize")
	return s.GenerateURL(key)
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

func (s *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
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
