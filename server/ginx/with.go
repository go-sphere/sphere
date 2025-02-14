package ginx

import (
	"errors"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"github.com/gin-gonic/gin"
	"net/http"
)

var internalServerError = errors.New("internal server error")

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

func AbortWithJsonError(ctx *gin.Context, code int, err error) {
	var hErr statusError
	if errors.As(err, &hErr) {
		ctx.AbortWithStatusJSON(hErr.Status(), ErrorResponse{
			Code:    hErr.Status(),
			Message: hErr.Error(),
		})
	} else {
		ctx.AbortWithStatusJSON(code, ErrorResponse{
			Code:    hErr.Status(),
			Message: err.Error(),
		})
	}
}

func WithRecover(message string, handler func(ctx *gin.Context)) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Warnw(
					message,
					logfields.Any("error", err),
				)
				AbortWithJsonError(ctx, http.StatusInternalServerError, internalServerError)
			}
		}()
		handler(ctx)
	}
}

func WithJson[T any](handler func(ctx *gin.Context) (T, error)) func(ctx *gin.Context) {
	return WithRecover("WithJson panic", func(ctx *gin.Context) {
		data, err := handler(ctx)
		if err != nil {
			AbortWithJsonError(ctx, http.StatusBadRequest, err)
		} else {
			ctx.JSON(200, DataResponse[T]{
				Success: true,
				Data:    data,
			})
		}
	})
}

func WithText(handler func(ctx *gin.Context) (string, error)) func(ctx *gin.Context) {
	return WithRecover("WithText panic", func(ctx *gin.Context) {
		data, err := handler(ctx)
		if err != nil {
			AbortWithJsonError(ctx, http.StatusBadRequest, err)
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
