package storage

import (
	"context"
	"io"
)

type FileUploadToken struct {
	Token string `json:"token"`
	Key   string `json:"key"`
	URL   string `json:"url"`
}

type FileUploadResult struct {
	Key string `json:"key"`
}

type URLHandler interface {
	GenerateURL(key string) string
	GenerateURLs(keys []string) []string
	GenerateImageURL(key string, width int) string
	ExtractKeyFromURL(uri string) string
	ExtractKeyFromURLWithMode(uri string, strict bool) (string, error)
}

type TokenGenerator interface {
	GenerateUploadToken(fileName string, dir string, nameBuilder func(filename string, dir ...string) string) FileUploadToken
}

type FileUploader interface {
	UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*FileUploadResult, error)
	UploadLocalFile(ctx context.Context, file string, key string) (*FileUploadResult, error)
}

type Storage interface {
	URLHandler
	FileUploader
	TokenGenerator
}
