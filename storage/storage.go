package storage

import (
	"context"
	"io"
)

// URLHandler provides URL generation and key extraction capabilities for storage backends.
// This interface enables URL-based access to stored files and reverse key lookup from URLs.
type URLHandler interface {
	// GenerateURL creates a public URL for accessing the file identified by the given key.
	GenerateURL(key string) string

	// GenerateURLs creates public URLs for multiple files in batch.
	GenerateURLs(keys []string) []string

	// ExtractKeyFromURL extracts the storage key from a given URL.
	ExtractKeyFromURL(uri string) string

	// ExtractKeyFromURLWithMode extracts the storage key from a URL with strict mode option.
	// When strict is true, returns an error if the URL format is invalid.
	ExtractKeyFromURLWithMode(uri string, strict bool) (string, error)
}

// ImageURLHandler extends URLHandler with image-specific URL generation capabilities.
// It provides resizing and transformation features for image files.
type ImageURLHandler interface {
	URLHandler
	// GenerateImageURL creates a URL for accessing an image with the specified width.
	// The height is typically auto-calculated to maintain aspect ratio.
	GenerateImageURL(key string, width int) string
}

// TokenGenerator provides secure upload token generation for client-side uploads.
// This is commonly used for direct-to-storage uploads from web browsers or mobile apps.
type TokenGenerator interface {
	// GenerateUploadToken creates a secure upload token, key, and URL for client uploads.
	// Returns [token, key, url] where token authorizes the upload operation.
	GenerateUploadToken(ctx context.Context, fileName string, dir string, nameBuilder func(filename string, dir ...string) string) ([3]string, error)
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

// FileDownloader provides file download and existence checking capabilities.
type FileDownloader interface {
	// IsFileExists checks whether a file exists in the storage backend.
	IsFileExists(ctx context.Context, key string) (bool, error)

	// DownloadFile retrieves a file from storage, returning the reader, MIME type, and size.
	// The caller is responsible for closing the returned ReadCloser.
	DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, int64, error) // reader, mime, size
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
	URLHandler
	FileDeleter
	FileUploader
	FileDownloader
	FileMoverCopier
}

// CDNStorage extends Storage with token generation for secure uploads.
// This interface is suitable for cloud storage backends that support direct client uploads.
type CDNStorage interface {
	Storage
	TokenGenerator
}

// ImageStorage combines CDN storage capabilities with image-specific URL handling.
// This interface is designed for backends that provide image processing and transformation.
type ImageStorage interface {
	ImageURLHandler
	CDNStorage
}
