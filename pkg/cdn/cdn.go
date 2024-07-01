package cdn

import (
	"context"
	"github.com/tbxark/go-base-api/pkg/cdn/model"
	"github.com/tbxark/go-base-api/pkg/cdn/qiniu"
	"io"
)

type UrlParser interface {
	RenderURL(key string) string
	RenderImageURL(key string, width int) string
	RenderURLs(keys []string) []string
	KeyFromURL(uri string) string
	KeyFromURLWithMode(uri string, strict bool) (string, error)
}

type Uploader interface {
	UploadToken(fileName string, dir string, nameBuilder func(fileName string, dir ...string) string) model.UploadToken
	UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*model.UploadResult, error)
	UploadLocalFile(ctx context.Context, file string, key string) (*model.UploadResult, error)
}

type CDN interface {
	UrlParser
	Uploader
}

// Config 修改这个结构体以更改要使用的CDN配置
type Config struct {
	*qiniu.Config
}

// NewCDN 修改这个函数的返回值以更改要使用的CDN实现
func NewCDN(config *Config) CDN {
	return qiniu.NewQiniu(config.Config)
}
