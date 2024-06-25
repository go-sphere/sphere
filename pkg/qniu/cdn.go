package qniu

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
	"time"
)

type Config struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`
	Dir       string `json:"dir"`
	Domain    string `json:"domain"`
	Host      string `json:"host"`
}

type UploadResponse struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

var (
	ErrInvalidURL = fmt.Errorf("invalid url")
)

type UploadKeyBuilder func(fileName string, dir ...string) string

func DefaultKeyBuilder(prefix string) UploadKeyBuilder {
	return func(fileName string, dir ...string) string {
		fileExt := path.Ext(fileName)
		sum := md5.Sum([]byte(fileName))
		nameMd5 := hex.EncodeToString(sum[:])
		name := strconv.Itoa(int(time.Now().Unix())) + "_" + nameMd5 + fileExt
		if prefix != "" {
			name = prefix + "_" + name
		}
		return path.Join(path.Join(dir...), name)
	}
}

func KeepFileNameKeyBuilder() UploadKeyBuilder {
	return func(fileName string, dir ...string) string {
		sum := md5.Sum([]byte(fileName))
		nameMd5 := hex.EncodeToString(sum[:])
		name := strconv.Itoa(int(time.Now().Unix())) + "_" + nameMd5
		return path.Join(path.Join(dir...), name, fileName)
	}
}

type CDN struct {
	mac    *qbox.Mac
	config *Config
}

func NewCDN(config *Config) *CDN {
	config.Domain = strings.TrimSuffix(config.Domain, "/")
	if config.Host == "" {
		u, err := url.Parse(config.Domain)
		if err == nil {
			config.Host = u.Host
		}
	}
	return &CDN{
		mac:    qbox.NewMac(config.AccessKey, config.SecretKey),
		config: config,
	}
}

func (n *CDN) RenderURL(key string) string {
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

func (n *CDN) RenderURLEx(key string, width int) string {
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

func (n *CDN) RenderURLs(keys []string) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		urls[i] = n.RenderURL(key)
	}
	return urls
}

func (n *CDN) KeyFromURLWithMode(uri string, strict bool) (string, error) {
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
			return "", ErrInvalidURL
		}
		return uri, nil
	}
	return strings.TrimPrefix(u.Path, "/"), nil
}

func (n *CDN) KeyFromURL(uri string) string {
	key, _ := n.KeyFromURLWithMode(uri, true)
	return key
}

type UploadToken struct {
	Token string `json:"token"`
	Key   string `json:"key"`
	URL   string `json:"url"`
}

func (n *CDN) GenUploadToken(fileName string, dir string, nameBuilder UploadKeyBuilder) UploadToken {
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
	return UploadToken{
		Token: put.UploadToken(n.mac),
		Key:   key,
		URL:   n.RenderURL(key),
	}
}

func (n *CDN) UploadFile(c context.Context, file io.Reader, size int64, key string) (*storage.PutRet, error) {
	put := &storage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := storage.Config{}
	ret := storage.PutRet{}
	formUploader := storage.NewFormUploader(&cfg)
	key = strings.TrimPrefix(key, "/")
	err := formUploader.Put(c, &ret, upToken, key, file, size, nil)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (n *CDN) UploadLocalFile(c context.Context, file string, key string) (*storage.PutRet, error) {
	put := &storage.PutPolicy{
		Scope: n.config.Bucket,
	}
	upToken := put.UploadToken(n.mac)
	cfg := storage.Config{}
	ret := storage.PutRet{}
	formUploader := storage.NewFormUploader(&cfg)
	key = strings.TrimPrefix(key, "/")
	err := formUploader.PutFile(c, &ret, upToken, key, file, nil)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}
