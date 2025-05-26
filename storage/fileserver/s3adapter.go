package fileserver

import (
	"bytes"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"

	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/storage"
	"github.com/google/uuid"
)

type S3Adapter struct {
	storage.Storage
	cache cache.ByteCache
}

func NewS3Adapter(cache cache.ByteCache, store storage.Storage) *S3Adapter {
	return &S3Adapter{
		Storage: store,
		cache:   cache,
	}
}

func (a *S3Adapter) CreateFileKey(ctx context.Context, filename string, expiration time.Duration) (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	err = a.cache.Set(ctx, id.String(), []byte(filename), expiration)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (a *S3Adapter) GenerateUploadToken(ctx context.Context, fileName string, dir string, nameBuilder func(filename string, dir ...string) string) ([3]string, error) {
	key := nameBuilder(fileName, dir)
	newToken, err := a.CreateFileKey(ctx, key, time.Minute*5)
	if err != nil {
		return [3]string{}, err
	}
	return [3]string{newToken, key, a.GenerateURL(key)}, nil
}

func (a *S3Adapter) RegisterPutFileUploader(route gin.IRouter) {
	route.PUT("/*key", func(ctx *gin.Context) {
		key := ctx.Param("key")
		if key == "" {
			ctx.AbortWithStatusJSON(400, gin.H{"error": "key is required"})
			return
		}
		filename, err := a.cache.Get(ctx, key)
		if err != nil {
			ctx.AbortWithStatusJSON(404, gin.H{"error": "upload token not unavailable"})
			return
		}
		if filename == nil {
			ctx.AbortWithStatusJSON(404, gin.H{"error": "upload token not found"})
			return
		}
		err = a.cache.Del(ctx, key)
		if err != nil {
			ctx.AbortWithStatusJSON(500, gin.H{"error": "refresh upload token failed"})
			return
		}
		data, err := ctx.GetRawData()
		if err != nil {
			ctx.AbortWithStatusJSON(500, gin.H{"error": "read file content failed"})
			return
		}
		uploadKey, err := a.UploadFile(ctx, bytes.NewReader(data), string(*filename))
		ctx.JSON(http.StatusOK, gin.H{
			"key": uploadKey,
			"url": a.Storage.GenerateURL(uploadKey),
		})
	})
}
