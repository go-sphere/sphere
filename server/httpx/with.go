package httpx

import (
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/core/errors/statuserr"
	"github.com/go-sphere/sphere/log"
)

var (
	errInternalServerPanic = errors.New("ServerError:PANIC")
)

// Value retrieves a typed value from the Gin context.
// It returns the value and a boolean indicating whether the key exists and the type matches.
func Value[T any](ctx Context, key string) (T, bool) {
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
func WithRecover(message string, handler func(ctx Context) error) Handler {
	return func(ctx Context) error {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf(
					message,
					log.Any("error", err),
				)
				AbortWithJsonError(ctx,
					statuserr.InternalServerError(
						errInternalServerPanic,
						fmt.Sprintf("internal server error: %v", err),
					),
				)
			}
		}()
		err := handler(ctx)
		if err != nil {
			AbortWithJsonError(ctx, err)
		}
		return nil
	}
}

// WithJson creates a Gin handler that returns JSON responses for typed data.
// It automatically handles errors by calling AbortWithJsonError and wraps successful
// responses in a standardized DataResponse structure.
func WithJson[T any](handler func(ctx Context) (T, error)) Handler {
	return WithRecover("WithJson panic", func(ctx Context) error {
		data, err := handler(ctx)
		if err != nil {
			return err
		}
		ctx.JSON(200, DataResponse[T]{
			Success: true,
			Data:    data,
		})
		return nil
	})
}

// WithText creates a Gin handler that returns plain text responses.
// It handles errors by calling AbortWithJsonError and returns successful
// string responses with HTTP 200 status.
func WithText(handler func(ctx Context) (string, error)) Handler {
	return WithRecover("WithText panic", func(ctx Context) error {
		data, err := handler(ctx)
		if err != nil {
			return err
		}
		ctx.Text(200, data)
		return nil
	})
}
