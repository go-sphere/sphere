package httpz

import (
	"errors"
	"net/http"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/core/errors/statuserr"
)

// ErrorParser is a function type that extracts error information for HTTP responses.
// It returns the error code, HTTP status code, and user-friendly message from an error.
type ErrorParser func(error) (int32, int32, string)

var defaultErrorParser ErrorParser = ParseError

// SetDefaultErrorParser sets the global error parser function for the package.
// This parser will be used by AbortWithJsonError when no specific parser is provided.
func SetDefaultErrorParser(parser ErrorParser) {
	defaultErrorParser = parser
}

// ParseError extracts error information from various error types.
// It recognizes StatusError, CodeError, and MessageError interfaces and falls back
// to defaults for unknown error types.
func ParseError(err error) (code int32, status int32, message string) {
	var he statuserr.HTTPError
	if errors.As(err, &he) {
		return he.GetCode(), he.GetStatus(), he.GetMessage()
	}
	var se statuserr.StatusError
	if errors.As(err, &se) {
		status = se.GetStatus()
	} else {
		status = http.StatusInternalServerError
	}
	var ce statuserr.CodeError
	if errors.As(err, &ce) {
		code = ce.GetCode()
	} else {
		code = status
	}
	var me statuserr.MessageError
	if errors.As(err, &me) {
		message = me.GetMessage()
	} else {
		message = err.Error()
	}
	return
}

// AbortWithJsonError terminates the request with a JSON error response.
// It uses the configured error parser to extract error details and ensures
// the HTTP status code is valid (200-599 range).
func AbortWithJsonError(ctx httpx.Context, err error) {
	code, status, message := defaultErrorParser(err)
	if status < 100 || status > 599 {
		status = http.StatusInternalServerError
	}
	ctx.JSON(int(status), ErrorResponse{
		Code:    int(code),
		Error:   err.Error(),
		Message: message,
	})
	ctx.Abort()
}
