// Package storage is the object-storage contract used by Sphere services:
// upload, download, delete, move, and copy, plus optional stat/list and CDN
// URL / upload-authorization capabilities.
//
// Drivers: local (filesystem), s3 (minio-go), qiniu (Kodo), kvcache (ByteCache
// as a blob store). fileserver is an HTTP adapter that wraps any Storage with
// one-time PUT tokens; it is not an S3 driver.
//
// Every driver normalizes keys via NormalizeKey before use, so a key stored
// by one backend addresses the same object on another. UploadFile returns the
// normalized key — persist that value, not the one passed in.
//
// Storage is FileDeleter + FileUploader + FileDownloader + FileMoverCopier.
// FileStater and FileLister are optional; probe with a type assertion.
// kvcache and fileserver do not implement them. CDNStorage adds URLHandler
// and UploadAuthorizer for public URLs and direct-to-storage uploads.
// UploadAuthRequest.TTL is a ceiling: a client cannot extend the driver's
// configured credential lifetime.
//
// DeleteFile is idempotent: missing keys succeed. URLHandler.GenerateURL is
// for public reads; private-bucket signed download URLs are out of scope.
package storage

import (
	"context"
	"io"
	"net/url"
	"time"
)

// URLHandler provides URL generation and key extraction capabilities for storage backends.
// This interface enables URL-based access to stored files and reverse key lookup from URLs.
type URLHandler interface {
	// GenerateURL creates a public URL for accessing the file identified by the given key.
	// Optional params are encoded into query string. When multiple values are provided,
	// only the first non-nil params is used.
	GenerateURL(key string, params ...url.Values) string

	// GenerateURLs creates public URLs for multiple files in batch.
	// Optional params are encoded into query string for each generated URL.
	GenerateURLs(keys []string, params ...url.Values) []string

	// ExtractKeyFromURL extracts the storage key from a given URL.
	ExtractKeyFromURL(uri string) string

	// ExtractKeyFromURLWithMode extracts the storage key from a URL with strict mode option.
	// When strict is true, returns an error if the URL format is invalid.
	ExtractKeyFromURLWithMode(uri string, strict bool) (string, error)
}

// UploadAuthorizationType indicates how upload authorization should be interpreted by clients.
type UploadAuthorizationType string

const (
	UploadAuthorizationTypeURL   UploadAuthorizationType = "url"
	UploadAuthorizationTypeToken UploadAuthorizationType = "token"
)

// UploadAuthorization carries the upload authorization data for client-side uploads.
type UploadAuthorization struct {
	Type   UploadAuthorizationType `json:"type" yaml:"type"`
	Value  string                  `json:"value" yaml:"value"`
	Method string                  `json:"method" yaml:"method"`
	// Headers is a reserved field for extra headers a client must send with the
	// upload request. No driver populates it today; it is kept intentionally as a
	// stable extension point, so do not remove it as "dead code".
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// UploadFileInfo contains the finalized storage information for an upload.
type UploadFileInfo struct {
	Key string `json:"key" yaml:"key"`
	URL string `json:"url" yaml:"url"`
}

// UploadAuthResult is the structured result for generating upload authorization.
type UploadAuthResult struct {
	Authorization UploadAuthorization `json:"authorization" yaml:"authorization"`
	File          UploadFileInfo      `json:"file" yaml:"file"`
}

// UploadNamingStrategy controls how upload file names are generated.
type UploadNamingStrategy string

const (
	UploadNamingStrategyRandomExt UploadNamingStrategy = "random_ext"
	UploadNamingStrategyHashExt   UploadNamingStrategy = "hash_ext"
	UploadNamingStrategyOriginal  UploadNamingStrategy = "original"
)

// UploadAuthRequest describes the input for upload authorization generation.
type UploadAuthRequest struct {
	FileName string `json:"file_name" yaml:"file_name"`
	Dir      string `json:"dir,omitempty" yaml:"dir,omitempty"`
	// TTL optionally shortens how long the generated authorization stays valid.
	// A zero value uses the driver's configured default TTL, which also caps this
	// field: a longer TTL is clamped down rather than honored, so populating it
	// from client input cannot extend a credential's lifetime.
	TTL time.Duration `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}

// UploadAuthorizer provides secure upload authorization generation for client-side uploads.
// This is commonly used for direct-to-storage uploads from web browsers or mobile apps.
type UploadAuthorizer interface {
	// GenerateUploadAuth creates upload authorization and target file information.
	GenerateUploadAuth(ctx context.Context, req UploadAuthRequest) (UploadAuthResult, error)
}

// FileUploader provides file upload capabilities to the storage backend.
type FileUploader interface {
	// UploadFile uploads data from a reader under key. The returned key is
	// NormalizeKey(key) and may differ from the argument; persist that value.
	UploadFile(ctx context.Context, file io.Reader, key string) (string, error)

	// UploadLocalFile uploads a local file to the storage backend with the specified key.
	// Returns the storage key or an error if upload fails.
	UploadLocalFile(ctx context.Context, file string, key string) (string, error)
}

// DownloadResult is the structured output for download operations.
type DownloadResult struct {
	Reader io.ReadCloser
	MIME   string
	Size   int64
}

// FileDownloader provides file download and existence checking capabilities.
type FileDownloader interface {
	// IsFileExists checks whether a file exists in the storage backend.
	IsFileExists(ctx context.Context, key string) (bool, error)

	// DownloadFile retrieves a file from storage.
	// The caller is responsible for closing DownloadResult.Reader.
	DownloadFile(ctx context.Context, key string) (DownloadResult, error)
}

// FileInfo carries lightweight object metadata returned by FileStater.
type FileInfo struct {
	// MIME is the content type of the object; it may be empty when the
	// backend does not record one.
	MIME string
	// Size is the object size in bytes.
	Size int64
}

// FileStater is an optional capability for cheaply retrieving object metadata
// (size and MIME type) without downloading the file body. Backends that expose
// a native stat/head operation implement it; callers should probe support with
// a type assertion since it is intentionally kept out of the core Storage
// interface.
type FileStater interface {
	// StatFile returns lightweight metadata for the file identified by key.
	// It returns storageerr.ErrNotFound when the key does not exist.
	StatFile(ctx context.Context, key string) (FileInfo, error)
}

// FileLister is an optional capability for enumerating stored keys under a
// prefix using cursor-based pagination. Callers should probe support with a
// type assertion; it is intentionally kept out of the core Storage interface.
type FileLister interface {
	// ListFiles returns up to limit keys under prefix, resuming after cursor
	// (pass an empty cursor for the first page). The returned next cursor is
	// non-empty when more keys may remain; feed it back on the following call.
	// An empty next cursor signals the end of the listing.
	//
	// Iterate until the next cursor is empty, not until a page comes back empty:
	// a backend may return a page with no keys while more remain. Treat the
	// cursor as opaque; some drivers return the last key of the page but others
	// return a backend-specific marker, so never construct one. limit is an
	// upper bound a driver may lower to its own maximum (Qiniu caps at 1000),
	// so a short page does not mean the listing is finished either.
	ListFiles(ctx context.Context, prefix, cursor string, limit int) (keys []string, next string, err error)
}

// FileDeleter provides file deletion capabilities.
type FileDeleter interface {
	// DeleteFile removes a file from the storage backend. Deletion is idempotent:
	// deleting a key that does not exist reports success rather than
	// storageerr.ErrNotFound.
	DeleteFile(ctx context.Context, key string) error
}

// FileMoverCopier provides file moving and copying operations within the storage backend.
type FileMoverCopier interface {
	// MoveFile relocates a file from source to destination key.
	// If overwrite is false, returns an error if destination already exists.
	MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error

	// CopyFile duplicates a file from source to destination key.
	// If overwrite is false, returns an error if destination already exists.
	CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error
}

// Storage combines the core file storage operations in a single interface.
// This is the primary interface for most file storage use cases.
type Storage interface {
	FileDeleter
	FileUploader
	FileDownloader
	FileMoverCopier
}

// CDNStorage extends Storage with token generation for secure uploads.
// This interface is suitable for cloud storage backends that support direct client uploads.
type CDNStorage interface {
	Storage
	URLHandler
	UploadAuthorizer
}
