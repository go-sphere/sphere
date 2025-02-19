package storage

import (
	"context"
	"github.com/TBXark/sphere/storage/models"
	"io"
)

type URLHandler interface {
	GenerateURL(key string) string
	GenerateURLs(keys []string) []string
	GenerateImageURL(key string, width int) string
	ExtractKeyFromURL(uri string) string
	ExtractKeyFromURLWithMode(uri string, strict bool) (string, error)
}

type TokenGenerator interface {
	GenerateUploadToken(fileName string, dir string, nameBuilder func(filename string, dir ...string) string) models.FileUploadToken
}

type FileUploader interface {
	UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*models.FileUploadResult, error)
	UploadLocalFile(ctx context.Context, file string, key string) (*models.FileUploadResult, error)
}

type Storage interface {
	URLHandler
	FileUploader
	TokenGenerator
}
