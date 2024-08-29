package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
	"net/http"
)

type DataResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type HttpStatusError interface {
	error
	Status() int
}

func GetValueFromContext[T any](key string, ctx *gin.Context) (*T, bool) {
	v, exists := ctx.Get(key)
	if !exists {
		return nil, false
	}
	if i, ok := v.(T); ok {
		return &i, true
	}
	return nil, false
}

func ResponseJsonError(ctx *gin.Context, err error) {
	var hErr HttpStatusError
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
					field.Any("error", err),
				)
				ctx.AbortWithStatusJSON(500, gin.H{
					"message": "internal server error",
				})
			}
		}()
		data, err := handler(ctx)
		if err != nil {
			ResponseJsonError(ctx, err)
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
					field.Any("error", err),
				)
				ctx.AbortWithStatusJSON(500, gin.H{
					"message": "internal server error",
				})
			}
		}()
		data, err := handler(ctx)
		if err != nil {
			ResponseJsonError(ctx, err)
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
					field.Any("error", err),
				)
				ctx.AbortWithStatusJSON(500, gin.H{
					"message": "internal server error",
				})
			}
		}()
		h.ServeHTTP(ctx.Writer, ctx.Request)
	}
}
