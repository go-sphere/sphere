package fileserver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/storage"
	"github.com/google/uuid"
)

var _ storage.CDNStorage = (*S3Adapter)(nil)

type Config struct {
	PublicBase string `json:"public_base" yaml:"public_base"`
	PutPrefix  string `json:"put_prefix" yaml:"put_prefix"`
}

type S3Adapter struct {
	storage.Storage

	config *Config
	cache  cache.ByteCache
}

func NewS3Adapter(config *Config, cache cache.ByteCache, store storage.Storage) *S3Adapter {
	return &S3Adapter{
		Storage: store,
		config:  config,
		cache:   cache,
	}
}

func (a *S3Adapter) createFileKey(ctx context.Context, filename string, expiration time.Duration) (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	err = a.cache.SetWithTTL(ctx, id.String(), []byte(filename), expiration)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (a *S3Adapter) GenerateUploadToken(ctx context.Context, fileName string, dir string, nameBuilder func(filename string, dir ...string) string) ([3]string, error) {
	key := nameBuilder(fileName, dir)
	newToken, err := a.createFileKey(ctx, key, time.Minute*5)
	if err != nil {
		return [3]string{}, err
	}
	uri, err := url.JoinPath(a.config.PublicBase, a.config.PutPrefix, newToken)
	if err != nil {
		return [3]string{}, err
	}
	return [3]string{
		uri,
		key,
		a.GenerateURL(key),
	}, nil
}

func (a *S3Adapter) RegisterPutFileUploader(route gin.IRouter, options ...UploadOption) {
	opts := newUploadOptions(options...)
	route.PUT("/:key", func(ctx *gin.Context) {
		key := ctx.Param("key")
		if key == "" {
			opts.abortWithError(ctx, http.StatusBadRequest, fmt.Errorf("key is required"))
			return
		}
		filename, found, err := a.cache.Get(ctx, key)
		if err != nil {
			opts.abortWithError(ctx, http.StatusBadRequest, err)
			return
		}
		if !found {
			opts.abortWithError(ctx, http.StatusBadRequest, fmt.Errorf("key expires or not found"))
			return
		}
		err = a.cache.Del(ctx, key)
		if err != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		data, err := ctx.GetRawData()
		if err != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		uploadKey, err := a.UploadFile(ctx, bytes.NewReader(data), string(filename))
		if err != nil {
			opts.abortWithError(ctx, http.StatusInternalServerError, err)
			return
		}
		opts.successWithData(ctx, uploadKey, a.GenerateURL(uploadKey))
	})
}

func (a *S3Adapter) RegisterFileDownloader(route gin.IRouter, options ...DownloaderOption) {
	RegisterFileDownloader(route, a.Storage, options...)
}
