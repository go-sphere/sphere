package ginx

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/TBXark/sphere/core/errors/statuserr"
	"github.com/TBXark/sphere/log"
	"github.com/gin-gonic/gin"
)

type Context = gin.Context

func Value[T any](key string, ctx *gin.Context) (T, bool) {
	v, exists := ctx.Get(key)
	var zero T
	if !exists {
		return zero, false
	}
	if i, ok := v.(T); ok {
		return i, true
	}
	return zero, false
}

func WithRecover(message string, handler func(ctx *gin.Context)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf(
					message,
					log.Any("error", err),
				)
				AbortWithJsonError(ctx,
					statuserr.InternalServerError(
						errors.New("ServerError:PANIC"),
						fmt.Sprintf("internal server error: %v", err),
					),
				)
			}
		}()
		handler(ctx)
	}
}

func WithJson[T any](handler func(ctx *gin.Context) (T, error)) gin.HandlerFunc {
	return WithRecover("WithJson panic", func(ctx *gin.Context) {
		data, err := handler(ctx)
		if err != nil {
			AbortWithJsonError(ctx, err)
		} else {
			ctx.JSON(200, DataResponse[T]{
				Success: true,
				Data:    data,
			})
		}
	})
}

func WithText(handler func(ctx *gin.Context) (string, error)) gin.HandlerFunc {
	return WithRecover("WithText panic", func(ctx *gin.Context) {
		data, err := handler(ctx)
		if err != nil {
			AbortWithJsonError(ctx, err)
		} else {
			ctx.String(200, data)
		}
	})
}

func WithHandler(h http.Handler) func(ctx *gin.Context) {
	return WithRecover("WithHandler panic", func(ctx *gin.Context) {
		h.ServeHTTP(ctx.Writer, ctx.Request)
	})
}
