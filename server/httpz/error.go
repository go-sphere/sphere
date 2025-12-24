package httpz

import (
	"net/http"

	"github.com/go-sphere/httpx"
)

// ErrorParser is a function type that extracts error information for HTTP responses.
// It returns the error code, HTTP status code, and user-friendly message from an error.
type ErrorParser func(error) (int32, int32, string)

var defaultErrorParser ErrorParser = httpx.ParseError

// SetDefaultErrorParser sets the global error parser function for the package.
// This parser will be used by AbortWithJsonError when no specific parser is provided.
func SetDefaultErrorParser(parser ErrorParser) {
	defaultErrorParser = parser
}

// AbortWithJsonError terminates the request with a JSON error response.
// It uses the configured error parser to extract error details and ensures
// the HTTP status code is valid (200-599 range).
func AbortWithJsonError(ctx httpx.Context, err error) {
	code, status, message := defaultErrorParser(err)
	if status < 100 || status > 599 {
		status = http.StatusInternalServerError
	}
	_ = ctx.JSON(int(status), ErrorResponse{
		Code:    int(code),
		Error:   err.Error(),
		Message: message,
	})
}
