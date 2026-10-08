package httpz

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-sphere/httpx"
)

// WithFormOptions contains configuration for file upload handling via multipart forms.
// Its fields are unexported; configure it only through WithFormOption values
// passed to WithFormFileReader or WithFormFileBytes.
type WithFormOptions struct {
	maxSize         int64
	fileFormKey     string
	allowExtensions map[string]struct{}
}

// WithFormOption is a functional option for configuring file upload behavior.
type WithFormOption func(*WithFormOptions)

func newWithFormOptions(opts ...WithFormOption) *WithFormOptions {
	defaults := &WithFormOptions{
		maxSize:     10 * 1024 * 1024, // 10MB
		fileFormKey: "file",
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// WithFormMaxSize sets the maximum file size allowed for uploads.
// The size is specified in bytes. It also bounds the request body: a declared
// Content-Length above maxSize plus 1 MiB of multipart overhead (boundaries,
// part headers and other form fields) is rejected with 413 before the body is
// parsed. A non-positive value disables both checks;
// combined with WithFormFileBytes that means an unbounded in-memory read, so
// only use it when the body is otherwise bounded.
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
			ext = strings.ToLower(strings.TrimSpace(ext))
			if ext == "" {
				continue
			}
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			options.allowExtensions[ext] = struct{}{}
		}
		if len(options.allowExtensions) == 0 {
			options.allowExtensions = nil
		}
	}
}

// WithFormFileReader wraps a multipart-upload handler as an httpx JSON handler.
// The inner handler receives an io.ReadSeekCloser (closed after return) and
// the original filename. Default max size is 10MiB; default form key is "file".
// Options are resolved once, when the handler is built.
//
// A request whose Content-Length exceeds the maximum size plus multipart
// overhead (see WithFormMaxSize) is rejected with 413 before the body is read,
// and a file larger than the maximum size (as reported by the multipart
// header) with 413 after parsing. A body without Content-Length (chunked) is
// parsed by the adapter before the size check, so put a transport-level body
// limit in front of endpoints that accept untrusted chunked uploads. A missing
// form field or malformed multipart body and a disallowed extension are
// rejected with 400. All rejections happen before the inner handler runs. The
// inner handler's result is rendered as described on WithJson.
func WithFormFileReader[T any](handler func(ctx httpx.Context, file io.ReadSeekCloser, filename string) (T, error), options ...WithFormOption) httpx.Handler {
	// The options are fixed once the handler is built, so resolve them here
	// rather than allocating a fresh set on every request.
	opts := newWithFormOptions(options...)
	return WithJson(func(ctx httpx.Context) (T, error) {
		var zero T
		if opts.maxSize > 0 {
			declared, err := strconv.ParseInt(ctx.Header("Content-Length"), 10, 64)
			if err == nil && declared > opts.maxSize+multipartOverhead {
				return zero, fileTooLargeError("")
			}
		}
		file, err := ctx.FormFile(opts.fileFormKey)
		if err != nil {
			// A missing field or malformed multipart body is the client's
			// fault; unclassified errors would render as 500.
			return zero, httpx.BadRequestError(err, "invalid multipart file upload")
		}
		if opts.maxSize > 0 && file.Size > opts.maxSize {
			return zero, fileTooLargeError(file.Filename)
		}
		if opts.allowExtensions != nil {
			ext := filepath.Ext(file.Filename)
			if _, ok := opts.allowExtensions[strings.ToLower(ext)]; !ok {
				return zero, httpx.BadRequestError(
					errors.New("FileError:FILE_EXTENSION_NOT_ALLOWED"),
					"File extension not allowed: "+ext,
				)
			}
		}
		read, err := file.Open()
		if err != nil {
			return zero, err
		}
		defer func() {
			_ = read.Close()
		}()
		return handler(ctx, read, file.Filename)
	})
}

// multipartOverhead is how far a multipart body may exceed the file size limit
// before its declared Content-Length is rejected unparsed.
const multipartOverhead = 1 << 20

func fileTooLargeError(filename string) error {
	message := "File size exceeds maximum allowed size"
	if filename != "" {
		message += ": " + filename
	}
	return httpx.WithStatus(http.StatusRequestEntityTooLarge, errors.New("FileError:FILE_TOO_LARGE"), message)
}

// WithFormFileBytes is WithFormFileReader that reads the whole file into memory
// before calling the inner handler. Prefer WithFormFileReader for large files.
// The same options, defaults, and 400/413 rejections apply.
func WithFormFileBytes[T any](handler func(ctx httpx.Context, file []byte, filename string) (T, error), options ...WithFormOption) httpx.Handler {
	return WithFormFileReader(func(ctx httpx.Context, file io.ReadSeekCloser, filename string) (T, error) {
		var zero T
		all, err := io.ReadAll(file)
		if err != nil {
			return zero, err
		}
		return handler(ctx, all, filename)
	}, options...)
}
