package fileserver

import (
	"context"
	"io"

	"github.com/TBXark/sphere/utils/safe"
	"github.com/gin-gonic/gin"
)

func WithFormFileReader[T any](handler func(ctx context.Context, file io.Reader, filename string) (*T, error)) func(ctx *gin.Context) (*T, error) {
	return func(ctx *gin.Context) (*T, error) {
		file, err := ctx.FormFile("file")
		if err != nil {
			return nil, err
		}
		read, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer safe.IfErrorPresent("close reader", read.Close)
		return handler(ctx, read, file.Filename)
	}
}

func WithFormFileBytes[T any](handler func(ctx context.Context, file []byte, filename string) (*T, error)) func(ctx *gin.Context) (*T, error) {
	return WithFormFileReader(func(ctx context.Context, file io.Reader, filename string) (*T, error) {
		all, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		return handler(ctx, all, filename)
	})
}
