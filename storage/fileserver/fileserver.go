package fileserver

import (
	"maps"
	"net/http"
	"os"
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
		if param == "" {
			abortWithError(ctx, http.StatusNotFound, os.ErrNotExist)
		}
		param = param[1:]
		reader, mime, size, err := storage.DownloadFile(ctx, param)
		if err != nil {
			abortWithError(ctx, http.StatusNotFound, err)
			return
		}
		defer safe.IfErrorPresent("close reader", reader.Close)
		headers := maps.Clone(sharedHeaders)
		ctx.DataFromReader(200, size, mime, reader, headers)
	})
}

type FileKeyBuilder func(ctx *gin.Context, filename string) string

func RegisterFormFileUploader(route gin.IRouter, storage storage.Storage, keyBuilder FileKeyBuilder) {
	route.POST("/", func(ctx *gin.Context) {
		file, err := ctx.FormFile("file")
		if err != nil {
			abortWithError(ctx, http.StatusBadRequest, err)
			return
		}
		read, err := file.Open()
		if err != nil {
			abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		defer safe.IfErrorPresent("close reader", read.Close)
		filename := keyBuilder(ctx, file.Filename)
		result, err := storage.UploadFile(ctx, read, filename)
		if err != nil {
			abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		successWithData(ctx, gin.H{
			"key": result,
			"url": storage.GenerateURL(result),
		})
	})
}

func abortWithError(ctx *gin.Context, status int, err error) {
	ctx.AbortWithStatusJSON(status, gin.H{"message": err.Error()})
}

func successWithData(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}
