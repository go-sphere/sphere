// Package httpz is the HTTP convention layer on top of github.com/go-sphere/httpx.
// Handlers return (T, error) or (string, error); wrappers recover panics, map
// errors to a JSON envelope, and wrap success in DataResponse[T].
//
// # Envelopes
//
// WithJson writes {"success": true, "data": T} at HTTP 200, or a status the
// handler set via ctx.Status when httpx.ResponseInfo is available (201, 204,
// …). Errors go through AbortWithJsonError: {"code": int, "message": string}
// at the parser's HTTP status. code is 0 unless the error implements
// httpx.CodeError. message is the generic status text unless the error
// implements httpx.MessageError with a non-empty message. ErrorResponse.Error
// is err.Error() only when SetDebugMode(true).
//
// The default parser is httpx.ParseError. SetDefaultErrorParser swaps it
// atomically; a nil parser is ignored.
//
// # Wrappers
//
// WithRecover, WithJson, WithText, WithFormFileReader, and WithFormFileBytes
// take and return httpx.Handler / httpx.Context, not Gin types. Panics become
// a 500 except http.ErrAbortHandler, which is re-panicked so net/http can
// drop the connection.
//
// EndpointsToMatches / MatchOperation turn generated [operation, method, path]
// routes into a matcher for middleware/selector.
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

// Value retrieves a typed value from the httpx context.
// It returns the value and whether the key exists and the type matches.
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

// WithRecover wraps an httpx handler with panic recovery.
// A panic is logged and turned into a JSON 500 except http.ErrAbortHandler,
// which is re-panicked so net/http can drop the connection.
func WithRecover(message string, handler func(ctx httpx.Context) error) httpx.Handler {
	return func(ctx httpx.Context) error {
		defer func() {
			if err := recover(); err != nil {
				// http.ErrAbortHandler is net/http's documented way for a
				// handler to abandon a request on purpose — most often because
				// the client went away mid-response. The server silently drops
				// the connection when it propagates. Swallowing it instead
				// turned every disconnect into a full stack trace at ERROR and
				// a 500 written to a connection that no longer exists.
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
// DataResponse[T] on success and AbortWithJsonError on failure.
func WithJson[T any](handler func(ctx httpx.Context) (T, error)) httpx.Handler {
	return WithRecover("WithJson panic", func(ctx httpx.Context) error {
		data, err := handler(ctx)
		if err != nil {
			return err
		}
		// Respect a status code the handler may have set via ctx.Status
		// (e.g. 201 Created, 204 No Content). httpx exposes the currently
		// buffered status through the optional ResponseInfo capability; when it
		// is unavailable or unset we fall back to 200 OK.
		status := http.StatusOK
		if ri, ok := httpx.AsResponseInfo(ctx); ok {
			if code := ri.StatusCode(); code >= 100 && code <= 599 {
				status = code
			}
		}
		return ctx.JSON(status, DataResponse[T]{
			Success: true,
			Data:    data,
		})
	})
}

// WithText wraps a (string, error) handler as an httpx handler that writes
// the string as text/plain on success and AbortWithJsonError on failure.
func WithText(handler func(ctx httpx.Context) (string, error)) httpx.Handler {
	return WithRecover("WithText panic", func(ctx httpx.Context) error {
		data, err := handler(ctx)
		if err != nil {
			return err
		}
		return ctx.Text(200, data)
	})
}
