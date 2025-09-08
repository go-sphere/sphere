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

type statusError struct {
	error
	status  int32
	code    int32
	message string
}

func (e *statusError) GetStatus() int32 {
	return e.status
}

func (e *statusError) GetCode() int32 {
	return e.code
}

func (e *statusError) GetMessage() string {
	return e.message
}

func (e *statusError) Error() string {
	return e.error.Error()
}

func (e *statusError) Unwrap() error {
	return e.error
}

// NewError creates a new error with HTTP status, custom code, and user message.
// If err is nil, a default HTTP error based on the status is used.
// This provides a comprehensive error structure for API responses.
func NewError(status, code int32, message string, err error) error {
	if err == nil {
		err = httpError(status)
	}
	return &statusError{
		error:   err,
		status:  status,
		code:    code,
		message: message,
	}
}

// JoinError creates a status error by wrapping an existing error with status and message.
// If the wrapped error implements CodeError, its code is preserved.
// This is useful for elevating internal errors to API-level errors.
func JoinError(status int32, message string, err error) error {
	if err == nil {
		err = httpError(status)
	}
	var code int32
	var codeError CodeError
	if errors.As(err, &codeError) {
		code = codeError.GetCode()
	} else {
		code = 0
	}
	return &statusError{
		error:   err,
		status:  status,
		code:    code,
		message: message,
	}
}

func WithStatus(status int32, err error, messages ...string) error {
	return JoinError(status, strings.Join(messages, "\n"), err)
}

func NewWithStatus(status int32, message string) error {
	return WithStatus(status, errors.New(message))
}

func BadRequestError(err error, messages ...string) error {
	return WithStatus(http.StatusBadRequest, err, messages...)
}

func NewBadRequestError(message string) error {
	return WithStatus(http.StatusBadRequest, errors.New(message))
}

func UnauthorizedError(err error, messages ...string) error {
	return WithStatus(http.StatusUnauthorized, err, messages...)
}

func NewUnauthorizedError(message string) error {
	return WithStatus(http.StatusUnauthorized, errors.New(message))
}

func ForbiddenError(err error, messages ...string) error {
	return WithStatus(http.StatusForbidden, err, messages...)
}

func NewForbiddenError(message string) error {
	return WithStatus(http.StatusForbidden, errors.New(message))
}

func NotFoundError(err error, messages ...string) error {
	return WithStatus(http.StatusNotFound, err, messages...)
}

func NewNotFoundError(message string) error {
	return WithStatus(http.StatusNotFound, errors.New(message))
}

func InternalServerError(err error, messages ...string) error {
	return WithStatus(http.StatusInternalServerError, err, messages...)
}

func NewInternalServerError(message string) error {
	return WithStatus(http.StatusInternalServerError, errors.New(message))
}

func httpError(status int32) error {
	msg := http.StatusText(int(status))
	if msg == "" {
		msg = "Unknown error"
	}
	return errors.New(msg)
}
