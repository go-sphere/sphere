package qiniu

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/TBXark/sphere/storage/urlhandler"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

type Config struct {
	AccessKey string `json:"access_key" yaml:"access_key"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`

	Bucket string `json:"bucket" yaml:"bucket"`
	Dir    string `json:"dir" yaml:"dir"`

	PublicBase string `json:"public_base" yaml:"public_base"`
}

type Client struct {
	urlhandler.Handler
	config *Config
	mac    *qbox.Mac
}

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

func (n *Client) GenerateImageURL(key string, width int) string {
	uri := n.GenerateURL(key)
	res, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	res.RawQuery = fmt.Sprintf("imageView2/2/w/%d/q/75", width)
	return res.String()
}

func (n *Client) keyPreprocess(key string) string {
	return strings.TrimPrefix(key, "/")
}

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

func (n *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key = n.keyPreprocess(key)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	_, err := manager.Stat(n.config.Bucket, key)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (n *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	key = n.keyPreprocess(key)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	object, err := manager.Get(n.config.Bucket, key, &storage.GetObjectInput{Context: ctx})
	if err != nil {
		return nil, "", 0, err
	}
	return object.Body, object.ContentType, object.ContentLength, nil
}

func (n *Client) DeleteFile(ctx context.Context, key string) error {
	key = n.keyPreprocess(key)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	err := manager.Delete(n.config.Bucket, key)
	if err != nil {
		return err
	}
	return nil
}

func (n *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = n.keyPreprocess(sourceKey)
	destinationKey = n.keyPreprocess(destinationKey)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	err := manager.Move(n.config.Bucket, sourceKey, n.config.Bucket, destinationKey, overwrite)
	if err != nil {
		return err
	}
	return nil
}

func (n *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey = n.keyPreprocess(sourceKey)
	destinationKey = n.keyPreprocess(destinationKey)
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	err := manager.Copy(n.config.Bucket, sourceKey, n.config.Bucket, destinationKey, overwrite)
	if err != nil {
		return err
	}
	return nil
}
