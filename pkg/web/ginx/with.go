package ginx

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"net/http"
)

type statusError interface {
	error
	Status() int
}

type Context = gin.Context

func Value[T any](key string, ctx *gin.Context) (*T, bool) {
	v, exists := ctx.Get(key)
	if !exists {
		return nil, false
	}
	if i, ok := v.(T); ok {
		return &i, true
	}
	return nil, false
}

func abortWithJsonError(ctx *gin.Context, err error) {
	var hErr statusError
	if errors.As(err, &hErr) {
		ctx.AbortWithStatusJSON(hErr.Status(), gin.H{
			"message": hErr.Error(),
		})
	} else {
		ctx.AbortWithStatusJSON(400, gin.H{
			"message": err.Error(),
		})
	}
}

func WithJson[T any](handler func(ctx *gin.Context) (T, error)) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Warnw(
					"WithJson panic",
					logfields.Any("error", err),
				)
				ctx.AbortWithStatusJSON(500, gin.H{
					"message": "internal server error",
				})
			}
		}()
		data, err := handler(ctx)
		if err != nil {
			abortWithJsonError(ctx, err)
		} else {
			ctx.JSON(200, gin.H{
				"success": true,
				"data":    data,
			})
		}
	}
}

func WithText(handler func(ctx *gin.Context) (string, error)) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Warnw(
					"WithText panic",
					logfields.Any("error", err),
				)
				ctx.AbortWithStatusJSON(500, gin.H{
					"message": "internal server error",
				})
			}
		}()
		data, err := handler(ctx)
		if err != nil {
			abortWithJsonError(ctx, err)
		} else {
			ctx.String(200, data)
		}
	}
}

func WithHandler(h http.Handler) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Warnw(
					"WithHandler panic",
					logfields.Any("error", err),
				)
				ctx.AbortWithStatusJSON(500, gin.H{
					"message": "internal server error",
				})
			}
		}()
		h.ServeHTTP(ctx.Writer, ctx.Request)
	}
}
