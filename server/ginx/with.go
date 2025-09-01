package ginx

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-sphere/sphere/core/errors/statuserr"
	"github.com/go-sphere/sphere/log"
)

// Context is a type alias for gin.Context for convenience.
type Context = gin.Context

// Value retrieves a typed value from the Gin context.
// It returns the value and a boolean indicating whether the key exists and the type matches.
func Value[T any](ctx *gin.Context, key string) (T, bool) {
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

// WithRecover wraps a Gin handler with panic recovery.
// If a panic occurs, it logs the error and returns a standardized internal server error response.
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

// WithJson creates a Gin handler that returns JSON responses for typed data.
// It automatically handles errors by calling AbortWithJsonError and wraps successful
// responses in a standardized DataResponse structure.
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

// WithText creates a Gin handler that returns plain text responses.
// It handles errors by calling AbortWithJsonError and returns successful
// string responses with HTTP 200 status.
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

// WithHandler wraps a standard http.Handler for use as a Gin handler function.
// It includes panic recovery and delegates the request/response handling to the wrapped handler.
func WithHandler(h http.Handler) func(ctx *gin.Context) {
	return WithRecover("WithHandler panic", func(ctx *gin.Context) {
		h.ServeHTTP(ctx.Writer, ctx.Request)
	})
}
