package fileserver

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/go-sphere/sphere/storage/urlhandler"
)

// Config holds the configuration for S3 adapter operations.
type Config struct {
	PutBase string        `json:"put_base" yaml:"put_base"`
	GetBase string        `json:"get_base" yaml:"get_base"`
	KeyTTL  time.Duration `json:"key_ttl" yaml:"key_ttl"`
}

// FileServer provides a caching layer and upload token generation for S3-compatible storage.
// It extends a base storage implementation with temporary upload URL generation capabilities.
type FileServer struct {
	opts    *options
	config  *Config
	cache   cache.ByteCache
	store   storage.Storage
	handler storage.URLHandler
}

// NewCDNAdapter creates a new CDN adapter with URL and token generation capabilities.
func NewCDNAdapter(config *Config, cache cache.ByteCache, store storage.Storage, options ...Option) (*FileServer, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if cache == nil {
		return nil, errors.New("cache is required")
	}
	if store == nil {
		return nil, errors.New("store is required")
	}
	if config.PutBase == "" {
		return nil, errors.New("put_base is required")
	}
	if config.GetBase == "" {
		return nil, errors.New("get_base is required")
	}
	if config.KeyTTL == 0 {
		config.KeyTTL = time.Minute * 5
	}
	handler, err := urlhandler.NewHandler(config.GetBase)
	if err != nil {
		return nil, err
	}
	opts := newOptions(options...)
	return &FileServer{
		opts:    opts,
		config:  config,
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

func (a *FileServer) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
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

// GenerateUploadToken creates a temporary upload URL and token for client-side uploads.
// Returns the upload URL, final storage key, and public access URL.
func (a *FileServer) GenerateUploadToken(ctx context.Context, fileName string, dir string, nameBuilder func(filename string, dir ...string) string) ([3]string, error) {
	if nameBuilder == nil {
		return [3]string{}, errors.New("nameBuilder is required")
	}
	key := nameBuilder(fileName, dir)
	newToken, err := a.opts.createFileKey(ctx, a, key)
	if err != nil {
		return [3]string{}, err
	}
	uri, err := url.JoinPath(a.config.PutBase, newToken)
	if err != nil {
		return [3]string{}, err
	}
	return [3]string{
		uri,
		key,
		a.GenerateURL(key),
	}, nil
}

func (a *FileServer) RegisterFileDownloader(route httpx.Router) {
	sharedHeaders := map[string]string{}
	if a.opts.downloadCacheControl != "" {
		sharedHeaders["Cache-Control"] = a.opts.downloadCacheControl
	}
	route.Handle(http.MethodGet, "/*filename", func(ctx httpx.Context) error {
		param := normalizeWildcardParam(ctx.Param("filename"))
		if param == "" {
			return httpx.NewNotFoundError("filename is required")
		}
		reader, mime, size, err := a.store.DownloadFile(ctx, param)
		if err != nil {
			if errors.Is(err, storageerr.ErrorNotFound) {
				return httpx.NotFoundError(err)
			}
			return httpx.InternalServerError(err)
		}
		defer func() {
			_ = reader.Close()
		}()
		headers := maps.Clone(sharedHeaders)
		for k, v := range headers {
			ctx.SetHeader(k, v)
		}
		return ctx.DataFromReader(200, mime, reader, int(size))
	})
}

func (a *FileServer) RegisterFileUploader(route httpx.Router) {
	route.Handle(http.MethodPut, "/:key", func(ctx httpx.Context) error {
		key := ctx.Param("key")
		if key == "" {
			return httpx.NewBadRequestError("key is required")
		}
		filename, found, err := a.cache.Get(ctx, key)
		if err != nil {
			return httpx.InternalServerError(err)
		}
		if !found {
			return httpx.NewBadRequestError("key expires or not found")
		}
		err = a.cache.Del(ctx, key)
		if err != nil {
			return httpx.InternalServerError(err)
		}
		data := ctx.BodyReader()
		if data == nil {
			return httpx.NewBadRequestError("empty request body")
		}
		uploadKey, err := a.UploadFile(ctx, data, string(filename))
		if err != nil {
			return httpx.InternalServerError(err)
		}
		return a.opts.uploadSuccessWithData(ctx, uploadKey, a.GenerateURL(uploadKey))
	})
}

func normalizeWildcardParam(raw string) string {
	return strings.TrimPrefix(raw, "/")
}
