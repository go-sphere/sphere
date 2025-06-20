package ginx

import (
	"errors"
	"github.com/TBXark/sphere/server/statuserr"
	"github.com/gin-gonic/gin"
	"io"
	"strings"
)

type WithFormOptions struct {
	maxSize         int64
	fileFormKey     string
	allowExtensions map[string]struct{}
}

type WithFormOption func(*WithFormOptions)

func newWithFormOptions(opts ...WithFormOption) *WithFormOptions {
	options := &WithFormOptions{
		maxSize:         10 * 1024 * 1024, // 10MB
		fileFormKey:     "file",
		allowExtensions: nil,
	}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func WithFormMaxSize(maxSize int64) WithFormOption {
	return func(options *WithFormOptions) {
		options.maxSize = maxSize
	}
}

func WithFormFileKey(key string) WithFormOption {
	return func(options *WithFormOptions) {
		options.fileFormKey = key
	}
}

func WithFormAllowExtensions(extensions ...string) WithFormOption {
	return func(options *WithFormOptions) {
		if options.allowExtensions == nil {
			options.allowExtensions = make(map[string]struct{}, len(extensions))
		}
		for _, ext := range extensions {
			options.allowExtensions[strings.ToLower(ext)] = struct{}{}
		}
		if len(options.allowExtensions) == 0 {
			options.allowExtensions = nil
		}
	}
}

func WithFormFileReader[T any](handler func(ctx *gin.Context, file io.Reader, filename string) (*T, error), options ...WithFormOption) gin.HandlerFunc {
	return WithJson(func(ctx *gin.Context) (*T, error) {
		opts := newWithFormOptions(options...)
		if opts.maxSize > 0 {
			if err := ctx.Request.ParseMultipartForm(opts.maxSize); err != nil {
				return nil, err
			}
		}
		file, err := ctx.FormFile(opts.fileFormKey)
		if err != nil {
			return nil, err
		}
		if opts.allowExtensions != nil {
			ext := file.Filename[strings.LastIndex(file.Filename, ".")+1:]
			if _, ok := opts.allowExtensions[strings.ToLower(ext)]; !ok {
				return nil, statuserr.BadRequestError(errors.New("extension not allowed"), "File extension not allowed: "+ext)
			}
		}
		read, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = read.Close()
		}()
		return handler(ctx, read, file.Filename)
	})
}

func WithFormFileBytes[T any](handler func(ctx *gin.Context, file []byte, filename string) (*T, error), options ...WithFormOption) gin.HandlerFunc {
	return WithFormFileReader(func(ctx *gin.Context, file io.Reader, filename string) (*T, error) {
		all, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		return handler(ctx, all, filename)
	}, options...)
}
