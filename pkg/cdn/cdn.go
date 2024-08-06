package cdn

import (
	"context"
	"github.com/tbxark/go-base-api/pkg/cdn/model"
	"io"
)

type UrlParser interface {
	RenderURL(key string) string
	RenderImageURL(key string, width int) string
	RenderURLs(keys []string) []string
	KeyFromURL(uri string) string
	KeyFromURLWithMode(uri string, strict bool) (string, error)
}

type Credential interface {
	UploadToken(fileName string, dir string, nameBuilder func(fileName string, dir ...string) string) model.UploadToken
}

type Uploader interface {
	UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*model.UploadResult, error)
	UploadLocalFile(ctx context.Context, file string, key string) (*model.UploadResult, error)
}

type CDN interface {
	UrlParser
	Uploader
	Credential
}
