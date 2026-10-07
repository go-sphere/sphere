package fileserver

import (
	"context"
	"strconv"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/google/uuid"
)

// UploadResult is the data payload of a successful upload response from the
// RegisterFileUploader endpoint.
type UploadResult struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

type options struct {
	createFileKey        func(ctx context.Context) (string, error)
	downloadCacheControl string
	inlineDownload       bool
	// ownsCache marks the cache as a resource of this FileServer rather than an
	// injected dependency, so Close releases it.
	ownsCache bool
}

// Option configures file server behavior.
type Option func(*options)

// WithCreateFileKey customizes how one-time upload tokens are generated.
// fn returns the token, which becomes the last path segment of the upload URL;
// the FileServer stores it in its token cache with the resolved TTL. The token
// must be unguessable and URL-path safe. A nil fn keeps the default, a random
// UUID.
func WithCreateFileKey(fn func(ctx context.Context) (string, error)) Option {
	return func(options *options) {
		if fn == nil {
			return
		}
		options.createFileKey = fn
	}
}

// WithCacheControl sets the Cache-Control header for downloaded files to
// "max-age=<maxAge>", with maxAge in seconds. Without it no Cache-Control
// header is set.
func WithCacheControl(maxAge uint64) Option {
	return func(o *options) {
		o.downloadCacheControl = "max-age=" + strconv.FormatUint(maxAge, 10)
	}
}

// WithOwnedCache makes the FileServer close the cache handed to NewCDNAdapter
// when FileServer.Close is called.
//
// By default the cache is injected and stays the caller's to close, because a
// Process-wide cache is usually shared with other users of the same server. A
// FileServer built on a cache it allocated for itself (typically an in-memory
// one, whose only owner is this adapter) must say so with this option:
// otherwise nothing ever releases the cache's background resources.
func WithOwnedCache() Option {
	return func(o *options) {
		o.ownsCache = true
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
		createFileKey: defaultCreateFileKey,
	}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

func defaultCreateFileKey(context.Context) (string, error) {
	// A random (v4) UUID keeps the one-time upload token path unpredictable,
	// matching the generator used in storage/utils.go.
	return uuid.NewString(), nil
}

func defaultUploadSuccessWithData(ctx httpx.Context, key, url string) error {
	// Success must be explicit: DataResponse.Success serializes without
	// omitempty, so a zero value would report a successful upload as
	// success:false, contradicting the envelope's own convention.
	return ctx.JSON(200, httpz.DataResponse[UploadResult]{Success: true, Data: UploadResult{Key: key, URL: url}})
}
