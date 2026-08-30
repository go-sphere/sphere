// Package fileserver is an HTTP adapter over any storage.Storage plus a
// cache.ByteCache of one-time PUT tokens (UUID → key, GetDel). It is not
// an S3 driver.
//
// PutBase and GetBase are required. KeyTTL of 0 becomes 5 minutes and is
// also the ceiling for UploadAuthRequest.TTL. Downloads set nosniff and
// Content-Disposition: attachment unless WithInlineDownload. Does not
// implement FileStater or FileLister.
package fileserver

import (
	"context"
	"errors"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/go-sphere/sphere/storage/urlhandler"
)

// Config holds HTTP adapter settings. PutBase and GetBase are required.
// KeyTTL of 0 becomes 5 minutes and is also the ceiling for
// UploadAuthRequest.TTL.
type Config struct {
	PutBase string `json:"put_base" yaml:"put_base"`
	GetBase string `json:"get_base" yaml:"get_base"`
	// KeyTTL is how long a one-time upload token stays valid, and also the
	// ceiling for UploadAuthRequest.TTL. A zero value falls back to defaultKeyTTL.
	KeyTTL       time.Duration                `json:"key_ttl" yaml:"key_ttl"`
	Dir          string                       `json:"dir" yaml:"dir"`
	UploadNaming storage.UploadNamingStrategy `json:"upload_naming" yaml:"upload_naming"`
}

// defaultKeyTTL is the one-time upload token validity used when Config.KeyTTL
// is unset.
const defaultKeyTTL = 5 * time.Minute

// FileServer is an HTTP adapter over any storage.Storage plus a
// cache.ByteCache of one-time PUT tokens. It is not an S3 driver.
type FileServer struct {
	opts    *options
	config  Config
	cache   cache.ByteCache
	store   storage.Storage
	handler storage.URLHandler
}

// NewCDNAdapter constructs a FileServer that wraps store with one-time PUT
// tokens stored in cache. PutBase, GetBase, cache, and store are required.
// It is not a CDN or S3 driver; the name is historical.
func NewCDNAdapter(conf Config, cache cache.ByteCache, store storage.Storage, options ...Option) (*FileServer, error) {
	if cache == nil {
		return nil, errors.New("cache is required")
	}
	if store == nil {
		return nil, errors.New("store is required")
	}
	if conf.PutBase == "" {
		return nil, errors.New("put_base is required")
	}
	if conf.GetBase == "" {
		return nil, errors.New("get_base is required")
	}
	if conf.KeyTTL == 0 {
		conf.KeyTTL = defaultKeyTTL
	}
	handler, err := urlhandler.NewHandler(conf.GetBase)
	if err != nil {
		return nil, err
	}
	opts := newOptions(options...)
	return &FileServer{
		opts:    opts,
		config:  conf,
		cache:   cache,
		store:   store,
		handler: handler,
	}, nil
}

func (a *FileServer) GenerateURL(key string, params ...url.Values) string {
	return a.handler.GenerateURL(key, params...)
}

func (a *FileServer) GenerateURLs(keys []string, params ...url.Values) []string {
	return a.handler.GenerateURLs(keys, params...)
}

func (a *FileServer) ExtractKeyFromURL(uri string) string {
	return a.handler.ExtractKeyFromURL(uri)
}

func (a *FileServer) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	return a.handler.ExtractKeyFromURLWithMode(uri, strict)
}

func (a *FileServer) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	return a.store.UploadFile(ctx, file, key)
}

func (a *FileServer) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	return a.store.UploadLocalFile(ctx, file, key)
}

func (a *FileServer) IsFileExists(ctx context.Context, key string) (bool, error) {
	return a.store.IsFileExists(ctx, key)
}

func (a *FileServer) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	return a.store.DownloadFile(ctx, key)
}

func (a *FileServer) DeleteFile(ctx context.Context, key string) error {
	return a.store.DeleteFile(ctx, key)
}

func (a *FileServer) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	return a.store.MoveFile(ctx, sourceKey, destinationKey, overwrite)
}

func (a *FileServer) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	return a.store.CopyFile(ctx, sourceKey, destinationKey, overwrite)
}

// GenerateUploadAuth creates temporary upload authorization for client-side uploads.
func (a *FileServer) GenerateUploadAuth(ctx context.Context, req storage.UploadAuthRequest) (storage.UploadAuthResult, error) {
	fileName, err := storage.BuildUploadFileName(req.FileName, a.config.UploadNaming)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key, err := storage.JoinUploadKey(a.config.Dir, req.Dir, fileName)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	// The configured token TTL is the ceiling; req.TTL may only shorten it.
	ttl := storage.ResolveUploadTTL(req.TTL, a.config.KeyTTL, defaultKeyTTL)
	newToken, err := a.opts.createFileKey(ctx, a, key, ttl)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	uri, err := url.JoinPath(a.config.PutBase, newToken)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	return storage.UploadAuthResult{
		Authorization: storage.UploadAuthorization{
			Type:   storage.UploadAuthorizationTypeURL,
			Value:  uri,
			Method: http.MethodPut,
		},
		File: storage.UploadFileInfo{
			Key: key,
			URL: a.GenerateURL(key),
		},
	}, nil
}

func (a *FileServer) RegisterFileDownloader(route httpx.Router) {
	sharedHeaders := map[string]string{}
	if a.opts.downloadCacheControl != "" {
		sharedHeaders["Cache-Control"] = a.opts.downloadCacheControl
	}
	// This endpoint serves user-uploaded bytes under a content type derived from
	// the key's extension, so .html and .svg come back as text/html and
	// image/svg+xml and execute on the origin serving GetBase. nosniff stops the
	// browser from upgrading an unknown or absent type into something
	// executable, and attachment disposition stops the declared type from
	// rendering at all. See WithInlineDownload for opting out.
	sharedHeaders["X-Content-Type-Options"] = "nosniff"
	path, param := httpx.FixWildcardPathIfNeed(route, "/*filename")
	route.Handle(http.MethodGet, path, func(ctx httpx.Context) error {
		filename := normalizeWildcardParam(ctx.Param(param))
		if filename == "" {
			return httpx.NewNotFoundError("filename is required")
		}
		result, err := a.store.DownloadFile(ctx.Context(), filename)
		if err != nil {
			if errors.Is(err, storageerr.ErrNotFound) {
				return httpx.NotFoundError(err)
			}
			return httpx.InternalServerError(err)
		}
		headers := maps.Clone(sharedHeaders)
		if !a.opts.inlineDownload {
			headers["Content-Disposition"] = contentDisposition(filename)
		}
		for k, v := range headers {
			ctx.SetHeader(k, v)
		}
		// result.Reader is expected to be closed by httpx.DataFromReader, so we don't close it here.
		return ctx.DataFromReader(200, result.MIME, result.Reader, int(result.Size))
	})
}

// contentDisposition builds an attachment disposition for key. The filename
// parameter is produced by mime.FormatMediaType, which quotes and encodes it;
// when that fails (an un-encodable name) the bare "attachment" is returned,
// since forcing the download matters and the name does not. Formatting it by
// hand would risk injecting a header value from a client-controlled key.
func contentDisposition(key string) string {
	name := path.Base(key)
	if name == "." || name == "/" {
		return "attachment"
	}
	if formatted := mime.FormatMediaType("attachment", map[string]string{"filename": name}); formatted != "" {
		return formatted
	}
	return "attachment"
}

func (a *FileServer) RegisterFileUploader(route httpx.Router) {
	route.Handle(http.MethodPut, "/:key", func(ctx httpx.Context) error {
		key := ctx.Param("key")
		if key == "" {
			return httpx.NewBadRequestError("key is required")
		}
		// GetDel consumes the token atomically. Reading and then deleting left a
		// window in which two concurrent requests both saw the token as valid,
		// so a single-use upload URL could be redeemed more than once.
		filename, found, err := a.cache.GetDel(ctx.Context(), key)
		if err != nil {
			return httpx.InternalServerError(err)
		}
		if !found {
			return httpx.NewBadRequestError("key expires or not found")
		}
		data := ctx.BodyReader()
		if data == nil {
			return httpx.NewBadRequestError("empty request body")
		}
		uploadKey, err := a.UploadFile(ctx.Context(), data, string(filename))
		if err != nil {
			return httpx.InternalServerError(err)
		}
		return a.opts.uploadSuccessWithData(ctx, uploadKey, a.GenerateURL(uploadKey))
	})
}

func normalizeWildcardParam(raw string) string {
	return strings.TrimPrefix(raw, "/")
}
