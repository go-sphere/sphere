package httpz

import (
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
)

var (
	errInternalServerPanic = errors.New("ServerError:PANIC")
)

// Value retrieves a typed value stored under key in the request-scoped
// httpx context store (ctx.Set / ctx.Get). It returns the zero value and false
// when the key is missing or the stored value is not a T.
func Value[T any](ctx httpx.Context, key string) (T, bool) {
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

// WithRecover wraps an httpx handler with panic recovery and error rendering.
// A non-nil error from handler is written with AbortWithJsonError. A panic is
// logged at Error level under message, with the panic value and stack, and
// written as a JSON 500 — except http.ErrAbortHandler, which is re-panicked so
// net/http can drop the connection. The returned handler always returns nil,
// so the engine's error handler never sees errors that WithRecover rendered.
func WithRecover(message string, handler func(ctx httpx.Context) error) httpx.Handler {
	return func(ctx httpx.Context) error {
		defer func() {
			if err := recover(); err != nil {
				// http.ErrAbortHandler is net/http's documented way to abandon a
				// request (usually a client disconnect); re-panic so the server
				// drops the connection instead of logging a stack and writing 500.
				if err == http.ErrAbortHandler {
					panic(err)
				}
				log.Error(
					message,
					log.Any("error", err),
					log.String("stack", string(debug.Stack())),
				)
				AbortWithJsonError(ctx,
					httpx.InternalServerError(
						errInternalServerPanic,
						"internal server error",
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

// WithJson wraps a (T, error) handler as an httpx handler that writes
// DataResponse[T] with Success set on success and AbortWithJsonError on
// failure. Panics are handled as described on WithRecover.
//
// The success status is 200 unless the handler set a status in the 200–599
// range through ctx.Status (for example 201). A status of 204 or 304 is
// written with no body instead of the envelope.
func WithJson[T any](handler func(ctx httpx.Context) (T, error)) httpx.Handler {
	return WithRecover("WithJson panic", func(ctx httpx.Context) error {
		data, err := handler(ctx)
		if err != nil {
			return err
		}
		// Respect a status code the handler may have set via ctx.Status
		// (e.g. 201 Created); StatusCode is part of the httpx.Context contract.
		// When the status is unset or out of range we fall back to 200 OK.
		// 1xx are never final statuses, and 204/304 forbid a body, so they are
		// written via NoContent instead of the JSON envelope.
		status := http.StatusOK
		if code := ctx.StatusCode(); code >= 200 && code <= 599 {
			status = code
		}
		if status == http.StatusNoContent || status == http.StatusNotModified {
			return ctx.NoContent(status)
		}
		return ctx.JSON(status, DataResponse[T]{
			Success: true,
			Data:    data,
		})
	})
}

// WithText wraps a (string, error) handler as an httpx handler that writes
// the string as a text response with status 200 on success and
// AbortWithJsonError on failure. Unlike WithJson it does not honor a status set
// through ctx.Status. Panics are handled as described on WithRecover.
func WithText(handler func(ctx httpx.Context) (string, error)) httpx.Handler {
	return WithRecover("WithText panic", func(ctx httpx.Context) error {
		data, err := handler(ctx)
		if err != nil {
			return err
		}
		return ctx.Text(200, data)
	})
}
