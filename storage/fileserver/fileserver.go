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
	"sync"
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
	// PutBase is the absolute URL where RegisterFileUploader's router is
	// mounted; upload URLs are PutBase joined with a one-time token.
	PutBase string `json:"put_base" yaml:"put_base"`
	// GetBase is the public base URL where RegisterFileDownloader's router is
	// mounted; GenerateURL joins keys onto it.
	GetBase string `json:"get_base" yaml:"get_base"`
	// KeyTTL is how long a one-time upload token stays valid, and also the
	// ceiling for UploadAuthRequest.TTL. A zero value falls back to defaultKeyTTL.
	KeyTTL time.Duration `json:"key_ttl" yaml:"key_ttl"`
	// Dir is the prefix directory for keys created by GenerateUploadAuth.
	Dir string `json:"dir" yaml:"dir"`
	// UploadNaming selects how GenerateUploadAuth names files; empty means
	// storage.UploadNamingStrategyRandomExt.
	UploadNaming storage.UploadNamingStrategy `json:"upload_naming" yaml:"upload_naming"`
}

// defaultKeyTTL is the one-time upload token validity used when Config.KeyTTL
// is unset.
const defaultKeyTTL = 5 * time.Minute

// FileServer is an HTTP adapter over any storage.Storage plus a
// cache.ByteCache of one-time PUT tokens. It is not an S3 driver.
//
// It implements storage.CDNStorage: the Storage methods delegate to the
// wrapped store unchanged, the URLHandler methods use Config.GetBase, and
// GenerateUploadAuth issues one-time upload URLs under Config.PutBase. It
// does not implement storage.FileStater or storage.FileLister even when the
// wrapped store does. Create it with NewCDNAdapter.
type FileServer struct {
	opts    *options
	config  Config
	cache   cache.ByteCache
	store   storage.Storage
	handler storage.URLHandler

	closeOnce sync.Once
	closeErr  error
}

// NewCDNAdapter constructs a FileServer that wraps store with one-time PUT
// tokens stored in cache. PutBase, GetBase, cache, and store are required;
// a missing one, or an unparsable GetBase, is an error. A zero KeyTTL becomes
// 5 minutes. The cache stays owned by the caller unless WithOwnedCache is
// given. It is not a CDN or S3 driver; the name is historical.
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

// Close releases the resources this FileServer owns: with WithOwnedCache it
// closes the injected cache, and without it Close does nothing, because the
// cache and the store belong to the caller. The store is never closed.
//
// Close is idempotent and safe to call concurrently; it reports the cache's
// own close error, and callers that keep using the adapter afterwards see the
// cache's closed error (cache.ErrClosed for the in-memory driver) on the
// upload-token path.
func (a *FileServer) Close() error {
	a.closeOnce.Do(func() {
		if a.opts.ownsCache {
			a.closeErr = a.cache.Close()
		}
	})
	return a.closeErr
}

// GenerateURL returns the public download URL for key under Config.GetBase;
// see urlhandler.Handler.GenerateURL. params are ignored.
func (a *FileServer) GenerateURL(key string, params ...url.Values) string {
	return a.handler.GenerateURL(key, params...)
}

// GenerateURLs returns GenerateURL for each key, in order.
func (a *FileServer) GenerateURLs(keys []string, params ...url.Values) []string {
	return a.handler.GenerateURLs(keys, params...)
}

// ExtractKeyFromURL extracts the key from a URL under Config.GetBase, or ""
// when the URL does not match; see urlhandler.Handler.ExtractKeyFromURL.
func (a *FileServer) ExtractKeyFromURL(uri string) string {
	return a.handler.ExtractKeyFromURL(uri)
}

// ExtractKeyFromURLWithMode is urlhandler.Handler.ExtractKeyFromURLWithMode
// against Config.GetBase.
func (a *FileServer) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	return a.handler.ExtractKeyFromURLWithMode(uri, strict)
}

// UploadFile delegates to the wrapped store's UploadFile.
func (a *FileServer) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	return a.store.UploadFile(ctx, file, key)
}

// UploadLocalFile delegates to the wrapped store's UploadLocalFile.
func (a *FileServer) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	return a.store.UploadLocalFile(ctx, file, key)
}

// IsFileExists delegates to the wrapped store's IsFileExists.
func (a *FileServer) IsFileExists(ctx context.Context, key string) (bool, error) {
	return a.store.IsFileExists(ctx, key)
}

// DownloadFile delegates to the wrapped store's DownloadFile; the caller
// closes the returned reader.
func (a *FileServer) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	return a.store.DownloadFile(ctx, key)
}

// DeleteFile delegates to the wrapped store's DeleteFile.
func (a *FileServer) DeleteFile(ctx context.Context, key string) error {
	return a.store.DeleteFile(ctx, key)
}

// MoveFile delegates to the wrapped store's MoveFile.
func (a *FileServer) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	return a.store.MoveFile(ctx, sourceKey, destinationKey, overwrite)
}

// CopyFile delegates to the wrapped store's CopyFile.
func (a *FileServer) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	return a.store.CopyFile(ctx, sourceKey, destinationKey, overwrite)
}

// GenerateUploadAuth creates temporary upload authorization for client-side uploads.
// It derives the key from req (Config.Dir, req.Dir, and the naming strategy),
// stores a one-time token for it in the cache, and returns
// Authorization{Type: URL, Method: PUT, Value: PutBase/<token>} plus the key
// and its GetBase URL. The token expires after Config.KeyTTL, or req.TTL if
// shorter. The client must then send the raw body to that URL, which only
// works once RegisterFileUploader is mounted at PutBase.
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

// RegisterFileDownloader registers GET /*filename on route, serving
// DownloadFile(filename) with its MIME type and size. Mount route at the path
// of Config.GetBase. Responses carry X-Content-Type-Options: nosniff and,
// unless WithInlineDownload is set, Content-Disposition: attachment; with
// WithCacheControl they also carry Cache-Control. A missing object answers
// 404 and an invalid key 400.
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
	// The named wildcard registers as-is on every adapter: those whose router
	// has no named wildcards rewrite it internally and keep Param("filename")
	// resolving. Calling FixWildcardPathIfNeed here and registering its result
	// was the old way, and is now wrong as well as redundant — the result is the
	// anonymous form, which httpx rejects at registration from v0.0.5 because
	// gin and hertz never accepted it and the three that did disagreed on the
	// parameter's key.
	route.Handle(http.MethodGet, "/*filename", func(ctx httpx.Context) error {
		filename := normalizeWildcardParam(ctx.Param("filename"))
		if filename == "" {
			return httpx.NewNotFoundError("filename is required")
		}
		result, err := a.store.DownloadFile(ctx.Context(), filename)
		if err != nil {
			if errors.Is(err, storageerr.ErrNotFound) {
				return httpx.NotFoundError(err)
			}
			// A traversal-shaped or otherwise invalid key is the client's
			// fault. The storage sentinels carry no HTTP status, so both
			// cases are mapped here explicitly rather than left to the
			// application's error parser.
			if errors.Is(err, storageerr.ErrFileNameInvalid) {
				return httpx.BadRequestError(err)
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
		return ctx.DataFromReader(200, result.MIME, result.Reader, result.Size)
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

// RegisterFileUploader registers PUT /:key on route, where key is a token
// issued by GenerateUploadAuth. Mount route at the path of Config.PutBase.
// The token is consumed on first use, even if the upload then fails; an
// unknown or expired token answers 400. On success the raw request body is
// stored under the authorized key and the response is a
// httpz.DataResponse[UploadResult] JSON envelope.
func (a *FileServer) RegisterFileUploader(route httpx.Router) {
	route.Handle(http.MethodPut, "/:key", func(ctx httpx.Context) error {
		key := ctx.Param("key")
		if key == "" {
			return httpx.NewBadRequestError("key is required")
		}
		// Validated before the token is spent: GetDel consumes it, and a request
		// that cannot be served anyway should not be the reason a client's upload
		// URL goes dead.
		body := ctx.BodyReader()
		if body == nil {
			return httpx.NewBadRequestError("empty request body")
		}
		// GetDel consumes the token atomically. Reading and then deleting left a
		// window in which two concurrent requests both saw the token as valid,
		// so a single-use upload URL could be redeemed more than once.
		//
		// The token is spent even when the upload below fails: a retry needs a new
		// authorization. That keeps a failed attempt from restoring a URL that a
		// caller could still be racing on, at the cost of one extra round trip.
		filename, found, err := a.cache.GetDel(ctx.Context(), key)
		if err != nil {
			return httpx.InternalServerError(err)
		}
		if !found {
			return httpx.NewBadRequestError("key expires or not found")
		}
		uploadKey, err := a.UploadFile(ctx.Context(), body, string(filename))
		if err != nil {
			return httpx.InternalServerError(err)
		}
		return defaultUploadSuccessWithData(ctx, uploadKey, a.GenerateURL(uploadKey))
	})
}

// normalizeWildcardParam strips the leading "/" a wildcard parameter can still
// carry for a doubled separator ("//a.png" matches with "/a.png"). httpx now
// strips it for an ordinary path on every adapter, and storage.NormalizeKey
// trims it again, so this does not change which object is served — it keeps the
// empty-filename guard above meaningful, which is the part that would move: a
// key of "/" would otherwise reach the store and come back as an invalid-name
// 400 instead of the 404 this endpoint answers when no filename was given.
func normalizeWildcardParam(raw string) string {
	return strings.TrimPrefix(raw, "/")
}
