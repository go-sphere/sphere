package httpz

import (
	"errors"
	"net/http"
	"sync/atomic"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
)

// ErrorParser is a function type that extracts error information for HTTP responses.
// It returns the error code, HTTP status code, and user-friendly message from an error.
type ErrorParser func(error) (int32, int32, string)

// Both globals are read by AbortWithJsonError on every request and may be written
// from application setup code, so they are swapped atomically rather than left as
// plain variables.
var (
	defaultErrorParser atomic.Pointer[ErrorParser]
	debugMode          atomic.Bool
)

func init() {
	var parser ErrorParser = httpx.ParseError
	defaultErrorParser.Store(&parser)
}

// SetDefaultErrorParser sets the global error parser function for the package.
// This parser will be used by AbortWithJsonError when no specific parser is provided.
// A nil parser is ignored, leaving the current one in place.
func SetDefaultErrorParser(parser ErrorParser) {
	if parser == nil {
		return
	}
	defaultErrorParser.Store(&parser)
}

// SetDebugMode controls whether raw error details are exposed to clients.
// When disabled (the default), the ErrorResponse.Error field is left empty and
// ErrorResponse.Message is replaced by the generic status text unless the error
// carries an explicit user-facing message, so that unclassified error strings
// and panic details are not leaked to callers. Enable it only in development to
// include err.Error() in responses.
func SetDebugMode(enabled bool) {
	debugMode.Store(enabled)
}

// DebugMode reports whether raw error details are exposed to clients.
func DebugMode() bool {
	return debugMode.Load()
}

// AbortWithJsonError writes an ErrorResponse and is the error path for
// WithJson/WithText/WithRecover. Status is clamped to 100–599 (invalid values
// become 500). A nil err logs a warning and writes a 500.
func AbortWithJsonError(ctx httpx.Context, err error) {
	if err == nil {
		log.Warn("AbortWithJsonError called with nil error")
		_ = ctx.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    0,
			Message: http.StatusText(http.StatusInternalServerError),
		})
		return
	}
	code, status, message := (*defaultErrorParser.Load())(err)
	if status < 100 || status > 599 {
		status = http.StatusInternalServerError
	}
	// Unify the semantics of ErrorResponse.Code: it carries an application
	// specific error code and is 0 when the error is unclassified. The default
	// parser (httpx.ParseError) falls back to code=status for errors that do not
	// implement httpx.CodeError, which conflates the HTTP status with the
	// application code. Normalize that here so unclassified errors always report
	// 0 and let the HTTP status carry the transport-level semantics.
	var ce httpx.CodeError
	if !errors.As(err, &ce) {
		code = 0
	}
	// Message is user-facing and always returned, so it must not become a second
	// channel for raw error text. httpx.ParseError falls back to err.Error() for
	// anything that does not implement httpx.MessageError, which puts driver and
	// database strings (credentials, hostnames, SQL) on the wire. Only an
	// explicit, non-empty message survives; everything else degrades to the
	// generic status text.
	var me httpx.MessageError
	if !errors.As(err, &me) || message == "" {
		message = http.StatusText(int(status))
	}
	resp := ErrorResponse{
		Code:    int(code),
		Message: message,
	}
	if debugMode.Load() {
		resp.Error = err.Error()
	}
	_ = ctx.JSON(int(status), resp)
}
