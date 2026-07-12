package httpz

import (
	"errors"
	"net/http"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
)

// ErrorParser is a function type that extracts error information for HTTP responses.
// It returns the error code, HTTP status code, and user-friendly message from an error.
type ErrorParser func(error) (int32, int32, string)

var defaultErrorParser ErrorParser = httpx.ParseError

// DebugMode controls whether raw error details are exposed to clients.
// When false (the default), the ErrorResponse.Error field is left empty so that
// unclassified error strings and panic details are not leaked to callers; the
// user-facing Message field (produced by the error parser) is always returned.
// Enable it only in development to include err.Error() in responses.
var DebugMode bool

// SetDefaultErrorParser sets the global error parser function for the package.
// This parser will be used by AbortWithJsonError when no specific parser is provided.
func SetDefaultErrorParser(parser ErrorParser) {
	defaultErrorParser = parser
}

// AbortWithJsonError terminates the request with a JSON error response.
// It uses the configured error parser to extract error details and ensures
// the HTTP status code is valid (200-599 range).
func AbortWithJsonError(ctx httpx.Context, err error) {
	if err == nil {
		log.Warn("AbortWithJsonError called with nil error")
		_ = ctx.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    0,
			Message: http.StatusText(http.StatusInternalServerError),
		})
		return
	}
	code, status, message := defaultErrorParser(err)
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
	resp := ErrorResponse{
		Code:    int(code),
		Message: message,
	}
	if DebugMode {
		resp.Error = err.Error()
	}
	_ = ctx.JSON(int(status), resp)
}
