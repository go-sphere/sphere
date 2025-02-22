package qiniu

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
)

var (
	ErrNotQiniuHost = fmt.Errorf("not qiniu host")
)

type Config struct {
	AccessKey string `json:"access_key" yaml:"access_key"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
	Bucket    string `json:"bucket" yaml:"bucket"`
	Dir       string `json:"dir" yaml:"dir"`
	Domain    string `json:"domain" yaml:"domain"`
	Host      string `json:"host" yaml:"host"`
}

type Client struct {
	config *Config
	mac    *qbox.Mac
}

func NewClient(config *Config) *Client {
	config.Domain = strings.TrimSuffix(config.Domain, "/")
	if config.Host == "" {
		u, err := url.Parse(config.Domain)
		if err == nil {
			config.Host = u.Host
		}
	}
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)
	return &Client{
		config: config,
		mac:    mac,
	}
}

func (n *Client) GenerateURL(key string) string {
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	buf := strings.Builder{}
	buf.WriteString(strings.TrimSuffix(n.config.Domain, "/"))
	buf.WriteString("/")
	buf.WriteString(strings.TrimPrefix(key, "/"))
	return buf.String()
}

func (n *Client) GenerateImageURL(key string, width int) string {
	// 判断是不是已经拼接了 ?imageView2 参数
	if strings.Contains(key, "?imageView2") {
		// 从URL中提取key
		key = n.ExtractKeyFromURL(key)
	}
	if key == "" {
		return ""
	}
	return n.GenerateURL(key) + "?imageView2/2/w/" + strconv.Itoa(width) + "/q/75"
}

func (n *Client) GenerateURLs(keys []string) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		urls[i] = n.GenerateURL(key)
	}
	return urls
}

func (n *Client) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	if uri == "" {
		return "", nil
	}
	// 不是 http 或者 https 开头的直接返回
	if !(strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")) {
		return strings.TrimPrefix(uri, "/"), nil
	}
	// 解析URL
	u, err := url.Parse(uri)
	if err != nil {
		return "", nil
	}
	// 不是以CDN域名开头的直接返回或者报错
	if u.Host != n.config.Host {
		if strict {
			return "", ErrNotQiniuHost
		}
		return uri, nil
	}
	return strings.TrimPrefix(u.Path, "/"), nil
}

func (n *Client) ExtractKeyFromURL(uri string) string {
	key, _ := n.ExtractKeyFromURLWithMode(uri, true)
	return key
}

func (n *Client) GenerateUploadToken(fileName string, dir string, nameBuilder func(fileName string, dir ...string) string) ([3]string, error) {
	fileExt := path.Ext(fileName)
	sum := md5.Sum([]byte(fileName))
	nameMd5 := hex.EncodeToString(sum[:])
	key := nameBuilder(nameMd5+fileExt, n.config.Dir, dir)
	key = strings.TrimPrefix(key, "/")
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

func (n *Client) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (string, error) {
	put := &storage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := storage.Config{}
	ret := storage.PutRet{}
	formUploader := storage.NewFormUploader(&cfg)
	key = strings.TrimPrefix(key, "/")
	err := formUploader.Put(ctx, &ret, upToken, key, file, size, nil)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}

func (n *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	put := &storage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := storage.Config{}
	ret := storage.PutRet{}
	formUploader := storage.NewFormUploader(&cfg)
	key = strings.TrimPrefix(key, "/")
	err := formUploader.PutFile(ctx, &ret, upToken, key, file, nil)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}

func (n *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, error) {
	manager := storage.NewBucketManager(n.mac, &storage.Config{})
	return manager.Get(n.config.Bucket, key, nil)
}
