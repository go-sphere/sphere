package storage

import (
	"context"
	"io"
	"net/url"
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
	Type    UploadAuthorizationType `json:"type" yaml:"type"`
	Value   string                  `json:"value" yaml:"value"`
	Method  string                  `json:"method" yaml:"method"`
	Headers map[string]string       `json:"headers,omitempty" yaml:"headers,omitempty"`
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
}

// UploadAuthorizer provides secure upload authorization generation for client-side uploads.
// This is commonly used for direct-to-storage uploads from web browsers or mobile apps.
type UploadAuthorizer interface {
	// GenerateUploadAuth creates upload authorization and target file information.
	GenerateUploadAuth(ctx context.Context, req UploadAuthRequest) (UploadAuthResult, error)
}

// FileUploader provides file upload capabilities to the storage backend.
type FileUploader interface {
	// UploadFile uploads data from a reader to the storage backend with the specified key.
	// Returns the storage key or an error if upload fails.
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

// FileDeleter provides file deletion capabilities.
type FileDeleter interface {
	// DeleteFile removes a file from the storage backend.
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
