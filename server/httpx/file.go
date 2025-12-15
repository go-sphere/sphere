package httpx

import (
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/core/errors/statuserr"
)

// WithFormOptions contains configuration for file upload handling via multipart forms.
type WithFormOptions struct {
	maxSize         int64
	fileFormKey     string
	allowExtensions map[string]struct{}
}

// WithFormOption is a functional option for configuring file upload behavior.
type WithFormOption func(*WithFormOptions)

func newWithFormOptions(opts ...WithFormOption) *WithFormOptions {
	defaults := &WithFormOptions{
		maxSize:         10 * 1024 * 1024, // 10MB
		fileFormKey:     "file",
		allowExtensions: nil,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// WithFormMaxSize sets the maximum file size allowed for uploads.
// The size is specified in bytes.
func WithFormMaxSize(maxSize int64) WithFormOption {
	return func(options *WithFormOptions) {
		options.maxSize = maxSize
	}
}

// WithFormFileKey sets the form field name for file uploads.
// The default field name is "file".
func WithFormFileKey(key string) WithFormOption {
	return func(options *WithFormOptions) {
		options.fileFormKey = key
	}
}

// WithFormAllowExtensions restricts file uploads to specific file extensions.
// Extensions are matched case-insensitively. If no extensions are provided,
// all file types are allowed.
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

// WithFormFileReader creates a Gin handler that processes uploaded files as io.ReadSeekCloser.
// It validates file size, extension constraints, and passes the file content to the handler function.
// The handler receives the file as an io.Reader along with the original filename.
func WithFormFileReader[T any](handler func(ctx httpx.Context, file io.ReadSeekCloser, filename string) (*T, error), options ...WithFormOption) httpx.Handler {
	return WithJson(func(ctx httpx.Context) (*T, error) {
		opts := newWithFormOptions(options...)
		file, err := ctx.FormFile(opts.fileFormKey)
		if err != nil {
			return nil, err
		}
		if opts.maxSize > 0 && file.Size > opts.maxSize {
			return nil, statuserr.BadRequestError(
				errors.New("FileError:FILE_TOO_LARGE"),
				"File size exceeds maximum allowed size: "+file.Filename,
			)
		}
		if opts.allowExtensions != nil {
			ext := filepath.Ext(file.Filename)
			if _, ok := opts.allowExtensions[strings.ToLower(ext)]; !ok {
				return nil, statuserr.BadRequestError(
					errors.New("FileError:FILE_EXTENSION_NOT_ALLOWED"),
					"File extension not allowed: "+ext,
				)
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

// WithFormFileBytes creates a Gin handler that processes uploaded files as byte arrays.
// It reads the entire file content into memory and passes it to the handler function.
// This is convenient for smaller files but should be used carefully with large files.
func WithFormFileBytes[T any](handler func(ctx httpx.Context, file []byte, filename string) (*T, error), options ...WithFormOption) httpx.Handler {
	return WithFormFileReader(func(ctx httpx.Context, file io.ReadSeekCloser, filename string) (*T, error) {
		all, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		return handler(ctx, all, filename)
	}, options...)
}
