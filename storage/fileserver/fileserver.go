package fileserver

import (
	"maps"
	"net/http"
	"os"
	"strconv"

	"github.com/TBXark/sphere/storage"
	"github.com/gin-gonic/gin"
)

type downloaderOptions struct {
	cacheControl   string
	abortWithError func(ctx *gin.Context, status int, err error)
}

type DownloaderOption func(o *downloaderOptions)

func newDownloaderOptions(opts ...DownloaderOption) *downloaderOptions {
	defaults := &downloaderOptions{
		cacheControl: "",
		abortWithError: func(ctx *gin.Context, status int, err error) {
			ctx.AbortWithStatusJSON(status, gin.H{
				"error": err.Error(),
			})
		},
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

func WithCacheControl(maxAge uint64) DownloaderOption {
	return func(o *downloaderOptions) {
		o.cacheControl = "max-age=" + strconv.FormatUint(maxAge, 10)
	}
}

func RegisterFileDownloader(route gin.IRouter, storage storage.Storage, options ...DownloaderOption) {
	opts := newDownloaderOptions(options...)
	sharedHeaders := map[string]string{}
	if opts.cacheControl != "" {
		sharedHeaders["Cache-Control"] = opts.cacheControl
	}
	route.GET("/*filename", func(ctx *gin.Context) {
		param := ctx.Param("filename")
		if param == "" {
			opts.abortWithError(ctx, http.StatusNotFound, os.ErrNotExist)
		}
		param = param[1:]
		reader, mime, size, err := storage.DownloadFile(ctx, param)
		if err != nil {
			opts.abortWithError(ctx, http.StatusNotFound, err)
			return
		}
		defer func() {
			_ = reader.Close
		}()
		headers := maps.Clone(sharedHeaders)
		ctx.DataFromReader(200, size, mime, reader, headers)
	})
}

type FileKeyBuilder func(ctx *gin.Context, filename string) string

type uploadOptions struct {
	abortWithError  func(ctx *gin.Context, status int, err error)
	successWithData func(ctx *gin.Context, key, url string)
}

type UploadOption func(*uploadOptions)

func newUploadOptions(opts ...UploadOption) *uploadOptions {
	defaults := &uploadOptions{
		abortWithError: func(ctx *gin.Context, status int, err error) {
			ctx.AbortWithStatusJSON(status, gin.H{
				"error": err.Error(),
			})
		},
		successWithData: func(ctx *gin.Context, key, url string) {
			ctx.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"key": key,
					"url": url,
				},
			})
		},
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

func RegisterFormFileUploader(route gin.IRouter, storage storage.Storage, keyBuilder FileKeyBuilder, options ...UploadOption) {
	opts := newUploadOptions(options...)
	route.POST("/", func(ctx *gin.Context) {
		file, err := ctx.FormFile("file")
		if err != nil {
			opts.abortWithError(ctx, http.StatusBadRequest, err)
			return
		}
		read, err := file.Open()
		if err != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		defer func() {
			_ = read.Close
		}()
		filename := keyBuilder(ctx, file.Filename)
		result, err := storage.UploadFile(ctx, read, filename)
		if err != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		opts.successWithData(ctx, result, storage.GenerateURL(result))
	})
}
