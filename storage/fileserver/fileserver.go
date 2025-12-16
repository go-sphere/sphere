package fileserver

import (
	"errors"
	"maps"
	"net/http"
	"strconv"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/storage"
)

// downloaderOptions holds configuration for file download operations.
type downloaderOptions struct {
	cacheControl   string
	abortWithError func(ctx httpx.Context, status int, err error)
}

// DownloaderOption configures file download behavior.
type DownloaderOption func(o *downloaderOptions)

func newDownloaderOptions(opts ...DownloaderOption) *downloaderOptions {
	defaults := &downloaderOptions{
		cacheControl: "",
		abortWithError: func(ctx httpx.Context, status int, err error) {
			ctx.JSON(status, httpx.H{
				"error": err.Error(),
			})
			ctx.Abort()
		},
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// WithCacheControl sets the Cache-Control header for downloaded files.
func WithCacheControl(maxAge uint64) DownloaderOption {
	return func(o *downloaderOptions) {
		o.cacheControl = "max-age=" + strconv.FormatUint(maxAge, 10)
	}
}

// RegisterFileDownloader registers a Gin route handler for file downloads from storage.
// It handles GET requests to serve files directly from the storage backend.
func RegisterFileDownloader(route httpx.Router, storage storage.Storage, options ...DownloaderOption) {
	opts := newDownloaderOptions(options...)
	sharedHeaders := map[string]string{}
	if opts.cacheControl != "" {
		sharedHeaders["Cache-Control"] = opts.cacheControl
	}
	route.Handle(http.MethodGet, "/*filename", func(ctx httpx.Context) {
		param := ctx.Param("filename")
		if param == "" {
			opts.abortWithError(ctx, http.StatusNotFound, errors.New("filename is required"))
			return
		}
		param = param[1:]
		reader, mime, size, err := storage.DownloadFile(ctx, param)
		if err != nil {
			opts.abortWithError(ctx, http.StatusNotFound, err)
			return
		}
		defer func() {
			_ = reader.Close
		}()
		headers := maps.Clone(sharedHeaders)
		for k, v := range headers {
			ctx.SetHeader(k, v)
		}
		ctx.DataFromReader(200, mime, reader, int(size))
	})
}

// FileKeyBuilder generates storage keys from HTTP context and filenames.
// This allows customization of how uploaded files are named and organized.
type FileKeyBuilder func(ctx httpx.Context, filename string) string

// uploadOptions holds configuration for file upload operations.
type uploadOptions struct {
	abortWithError  func(ctx httpx.Context, status int, err error)
	successWithData func(ctx httpx.Context, key, url string)
}

// UploadOption configures file upload behavior and response handling.
type UploadOption func(*uploadOptions)

func newUploadOptions(opts ...UploadOption) *uploadOptions {
	defaults := &uploadOptions{
		abortWithError: func(ctx httpx.Context, status int, err error) {
			ctx.JSON(status, httpx.H{
				"error": err.Error(),
			})
			ctx.Abort()
		},
		successWithData: func(ctx httpx.Context, key, url string) {
			ctx.JSON(http.StatusOK, httpx.H{
				"success": true,
				"data": httpx.H{
					"key": key,
					"url": url,
				},
			})
		},
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// RegisterFormFileUploader registers a Gin route handler for form-based file uploads.
// It accepts multipart form uploads and stores files using the provided key builder.
func RegisterFormFileUploader(route httpx.Router, storage storage.Storage, keyBuilder FileKeyBuilder, options ...UploadOption) {
	opts := newUploadOptions(options...)
	route.Handle(http.MethodPost, "/", func(ctx httpx.Context) {
		file, err := ctx.FormFile("file")
		if err != nil {
			opts.abortWithError(ctx, http.StatusBadRequest, err)
			return
		}
		read, err := file.Open()
		if err != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		defer func() {
			_ = read.Close
		}()
		filename := keyBuilder(ctx, file.Filename)
		result, err := storage.UploadFile(ctx, read, filename)
		if err != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		opts.successWithData(ctx, result, storage.GenerateURL(result))
	})
}
