package cdn

import (
	"context"
	"github.com/tbxark/go-base-api/pkg/cdn/models"
	"io"
)

type UrlParser interface {
	RenderURL(key string) string
	RenderURLs(keys []string) []string
	KeyFromURL(uri string) string
	KeyFromURLWithMode(uri string, strict bool) (string, error)
}

type ImageProcessor interface {
	RenderImageURL(key string, width int) string
}

type Credential interface {
	UploadToken(fileName string, dir string, nameBuilder func(fileName string, dir ...string) string) models.UploadToken
}

type Uploader interface {
	UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*models.UploadResult, error)
	UploadLocalFile(ctx context.Context, file string, key string) (*models.UploadResult, error)
}

type CDN interface {
	UrlParser
	ImageProcessor
	Uploader
	Credential
}
