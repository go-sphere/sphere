package cdn

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strconv"
	"time"
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

var (
	ErrInvalidURL = fmt.Errorf("invalid url")
)

type UploadToken struct {
	Token string `json:"token"`
	Key   string `json:"key"`
	URL   string `json:"url"`
}

type UploadResult struct {
	Key string `json:"key"`
}

type UrlParser interface {
	RenderURL(key string) string
	RenderImageURL(key string, width int) string
	RenderURLs(keys []string) []string
	KeyFromURL(uri string) string
	KeyFromURLWithMode(uri string, strict bool) (string, error)
}

type Uploader interface {
	UploadToken(fileName string, dir string, nameBuilder UploadKeyBuilder) UploadToken
	UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*UploadResult, error)
	UploadLocalFile(ctx context.Context, file string, key string) (*UploadResult, error)
}

type CDN interface {
	UrlParser
	Uploader
}
