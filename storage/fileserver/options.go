package fileserver

import (
	"context"
	"strconv"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/google/uuid"
)

type UploadResult struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

type options struct {
	uploadSuccessWithData func(ctx httpx.Context, key, url string) error
	createFileKey         func(ctx context.Context, server *FileServer, filename string) (string, error)
	downloadCacheControl  string
}

// Option configures file server behavior.
type Option func(*options)

// WithCreateFileKey customizes temporary upload key generation behavior.
func WithCreateFileKey(fn func(ctx context.Context, server *FileServer, filename string) (string, error)) Option {
	return func(options *options) {
		if fn == nil {
			return
		}
		options.createFileKey = fn
	}
}

// WithCacheControl sets the Cache-Control header for downloaded files.
func WithCacheControl(maxAge uint64) Option {
	return func(o *options) {
		o.downloadCacheControl = "max-age=" + strconv.FormatUint(maxAge, 10)
	}
}

func newOptions(opts ...Option) *options {
	opt := &options{
		uploadSuccessWithData: defaultUploadSuccessWithData,
		createFileKey:         defaultCreateFileKey,
	}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

func defaultCreateFileKey(ctx context.Context, server *FileServer, filename string) (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	err = server.cache.SetWithTTL(ctx, id.String(), []byte(filename), server.config.KeyTTL)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func defaultUploadSuccessWithData(ctx httpx.Context, key, url string) error {
	return ctx.JSON(200, httpz.DataResponse[UploadResult]{Data: UploadResult{Key: key, URL: url}})
}
