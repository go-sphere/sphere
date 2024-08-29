package qiniu

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/tbxark/go-base-api/pkg/cdn/models"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type Config struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`
	Dir       string `json:"dir"`
	Domain    string `json:"domain"`
	Host      string `json:"host"`
}

type Qiniu struct {
	mac    *qbox.Mac
	config *Config
}

func NewQiniu(config *Config) *Qiniu {
	config.Domain = strings.TrimSuffix(config.Domain, "/")
	if config.Host == "" {
		u, err := url.Parse(config.Domain)
		if err == nil {
			config.Host = u.Host
		}
	}
	return &Qiniu{
		mac:    qbox.NewMac(config.AccessKey, config.SecretKey),
		config: config,
	}
}

func (n *Qiniu) RenderURL(key string) string {
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

func (n *Qiniu) RenderImageURL(key string, width int) string {
	// 判断是不是已经拼接了 ?imageView2 参数
	if strings.Contains(key, "?imageView2") {
		// 从URL中提取key
		key = n.KeyFromURL(key)
	}
	if key == "" {
		return ""
	}
	return n.RenderURL(key) + "?imageView2/2/w/" + strconv.Itoa(width) + "/q/75"
}

func (n *Qiniu) RenderURLs(keys []string) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		urls[i] = n.RenderURL(key)
	}
	return urls
}

func (n *Qiniu) KeyFromURLWithMode(uri string, strict bool) (string, error) {
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
			return "", fmt.Errorf("invalid url")
		}
		return uri, nil
	}
	return strings.TrimPrefix(u.Path, "/"), nil
}

func (n *Qiniu) KeyFromURL(uri string) string {
	key, _ := n.KeyFromURLWithMode(uri, true)
	return key
}

func (n *Qiniu) UploadToken(fileName string, dir string, nameBuilder func(fileName string, dir ...string) string) models.UploadToken {
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
	return models.UploadToken{
		Token: put.UploadToken(n.mac),
		Key:   key,
		URL:   n.RenderURL(key),
	}
}

func (n *Qiniu) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*models.UploadResult, error) {
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
		return nil, err
	}
	return &models.UploadResult{
		Key: ret.Key,
	}, nil
}

func (n *Qiniu) UploadLocalFile(ctx context.Context, file string, key string) (*models.UploadResult, error) {
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
		return nil, err
	}
	return &models.UploadResult{
		Key: ret.Key,
	}, nil
}
