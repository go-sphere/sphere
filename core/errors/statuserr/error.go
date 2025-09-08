package statuserr

import (
	"errors"
	"net/http"
	"strings"
)

// StatusError represents an error that carries an HTTP status code.
// This interface allows errors to be categorized by their HTTP semantics.
type StatusError interface {
	error
	// GetStatus returns the HTTP status code associated with this error.
	GetStatus() int32
}

// CodeError represents an error that carries a custom error code.
// This is useful for application-specific error classification beyond HTTP status.
type CodeError interface {
	error
	// GetCode returns the custom error code associated with this error.
	GetCode() int32
}

// MessageError represents an error that carries a user-friendly message.
// This allows separation between technical error details and user-facing messages.
type MessageError interface {
	error
	// GetMessage returns the user-friendly message associated with this error.
	GetMessage() string
}

// HTTPError is a comprehensive error type that includes HTTP status, custom code, and user message.
type HTTPError interface {
	error
	StatusError
	CodeError
	MessageError
}

// httpError is an unexported concrete implementation of HTTPError.
// Keeping it unexported prevents external code from depending on the
// concrete type, preserving flexibility to change internals.
type httpError struct {
	error
	status  int32
	code    int32
	message string
}

func (e *httpError) GetStatus() int32 {
	return e.status
}

func (e *httpError) GetCode() int32 {
	return e.code
}

func (e *httpError) GetMessage() string {
	return e.message
}

func (e *httpError) Error() string {
	return e.error.Error()
}

func (e *httpError) Unwrap() error {
	return e.error
}

// NewError creates a new error with HTTP status, custom code, and user message.
// It returns the broader HTTPError interface to avoid exposing the concrete type
// and keep the API surface stable. If err is nil, a default HTTP error based on the
// status is used.
func NewError(status, code int32, message string, err error) HTTPError {
	if err == nil {
		err = httpStatusError(status)
	}
	return &httpError{
		error:   err,
		status:  status,
		code:    code,
		message: message,
	}
}

func WithStatus(status int32, err error, messages ...string) HTTPError {
	code := int32(0)
	var se CodeError
	if errors.As(err, &se) {
		code = se.GetCode()
	}
	return NewError(status, code, strings.Join(messages, "; "), err)
}

func NewWithStatus(status int32, message string) HTTPError {
	return WithStatus(status, errors.New(message))
}

func BadRequestError(err error, messages ...string) HTTPError {
	return WithStatus(http.StatusBadRequest, err, messages...)
}

func NewBadRequestError(message string) HTTPError {
	return WithStatus(http.StatusBadRequest, errors.New(message))
}

func UnauthorizedError(err error, messages ...string) HTTPError {
	return WithStatus(http.StatusUnauthorized, err, messages...)
}

func NewUnauthorizedError(message string) HTTPError {
	return WithStatus(http.StatusUnauthorized, errors.New(message))
}

func ForbiddenError(err error, messages ...string) HTTPError {
	return WithStatus(http.StatusForbidden, err, messages...)
}

func NewForbiddenError(message string) HTTPError {
	return WithStatus(http.StatusForbidden, errors.New(message))
}

func NotFoundError(err error, messages ...string) HTTPError {
	return WithStatus(http.StatusNotFound, err, messages...)
}

func NewNotFoundError(message string) HTTPError {
	return WithStatus(http.StatusNotFound, errors.New(message))
}

func InternalServerError(err error, messages ...string) HTTPError {
	return WithStatus(http.StatusInternalServerError, err, messages...)
}

func NewInternalServerError(message string) HTTPError {
	return WithStatus(http.StatusInternalServerError, errors.New(message))
}

func httpStatusError(status int32) error {
	msg := http.StatusText(int(status))
	if msg == "" {
		msg = "Unknown error"
	}
	return errors.New(msg)
}
