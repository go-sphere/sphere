package fileserver

import (
	"net/http"
	"strconv"

	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/utils/safe"
	"github.com/gin-gonic/gin"
)

type DownloaderOptions struct {
	cacheControl string
}

type DownloaderOption func(o *DownloaderOptions)

func WithCacheControl(maxAge uint64) DownloaderOption {
	return func(o *DownloaderOptions) {
		o.cacheControl = "max-age=" + strconv.FormatUint(maxAge, 10)
	}
}

func RegisterFileDownloader(route gin.IRouter, storage storage.Storage, options ...DownloaderOption) {
	opts := &DownloaderOptions{}
	for _, opt := range options {
		opt(opts)
	}
	sharedHeaders := map[string]string{}
	if opts.cacheControl != "" {
		sharedHeaders["Cache-Control"] = opts.cacheControl
	}
	route.GET("/*filename", func(ctx *gin.Context) {
		param := ctx.Param("filename")
		reader, mime, size, err := storage.DownloadFile(ctx, param)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		defer safe.IfErrorPresent("close reader", reader.Close)
		ctx.DataFromReader(200, size, mime, reader, sharedHeaders)
	})
}

type FileKeyBuilder func(ctx *gin.Context, filename string) string

func RegisterFormFileUploader(route gin.IRouter, storage storage.Storage, keyBuilder FileKeyBuilder) {
	route.POST("/", func(ctx *gin.Context) {
		file, err := ctx.FormFile("file")
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		read, err := file.Open()
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		defer safe.IfErrorPresent("close reader", read.Close)
		filename := keyBuilder(ctx, file.Filename)
		result, err := storage.UploadFile(ctx, read, filename)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"key": result,
			"url": storage.GenerateURL(result),
		})
	})
}
