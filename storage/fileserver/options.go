package fileserver

import (
	"context"
	"strconv"
	"time"

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
	createFileKey         func(ctx context.Context, server *FileServer, filename string, ttl time.Duration) (string, error)
	downloadCacheControl  string
	inlineDownload        bool
}

// Option configures file server behavior.
type Option func(*options)

// WithCreateFileKey customizes temporary upload key generation behavior.
// The ttl argument is the resolved token validity (request TTL when provided,
// otherwise the configured KeyTTL); custom implementations should honor it when
// persisting the token.
func WithCreateFileKey(fn func(ctx context.Context, server *FileServer, filename string, ttl time.Duration) (string, error)) Option {
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

// WithInlineDownload serves downloads inline instead of as attachments.
//
// The download endpoint serves whatever a client uploaded, and the content type
// is derived from the key's extension, so an uploaded .html or .svg is returned
// as text/html or image/svg+xml and its script runs on the origin serving
// GetBase. Attachment disposition is therefore the default. Enable inline only
// when GetBase is an origin that carries no session — a dedicated asset domain
// or CDN host — or when the upload path is restricted to types that cannot
// carry script.
func WithInlineDownload() Option {
	return func(o *options) {
		o.inlineDownload = true
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

func defaultCreateFileKey(ctx context.Context, server *FileServer, filename string, ttl time.Duration) (string, error) {
	// Use a random (v4) UUID for the one-time upload token path so it is not
	// predictable, matching the generator used in storage/utils.go.
	id := uuid.NewString()
	err := server.cache.SetWithTTL(ctx, id, []byte(filename), ttl)
	if err != nil {
		return "", err
	}
	return id, nil
}

func defaultUploadSuccessWithData(ctx httpx.Context, key, url string) error {
	return ctx.JSON(200, httpz.DataResponse[UploadResult]{Data: UploadResult{Key: key, URL: url}})
}
