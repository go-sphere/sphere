package statuserr

import (
	"errors"
	"net/http"
	"strings"
)

type StatusError interface {
	error
	GetStatus() int32
}

type CodeError interface {
	error
	GetCode() int32
}

type MessageError interface {
	error
	GetMessage() string
}

type Error struct {
	error
	status  int32
	code    int32
	message string
}

func NewError(status, code int32, message string, err error) error {
	if err == nil {
		err = httpError(status)
	}
	return &Error{
		error:   err,
		status:  status,
		code:    code,
		message: message,
	}
}

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
	return &Error{
		error:   err,
		status:  status,
		code:    code,
		message: message,
	}
}

func (e *Error) GetStatus() int32 {
	return e.status
}

func (e *Error) GetCode() int32 {
	return e.code
}

func (e *Error) GetMessage() string {
	return e.message
}

func (e *Error) Error() string {
	return e.error.Error()
}

func (e *Error) Unwrap() error {
	return e.error
}

func WithStatus(status int32, err error, messages ...string) error {
	return JoinError(status, strings.Join(messages, ", "), err)
}

func BadRequestError(err error, messages ...string) error {
	return WithStatus(http.StatusBadRequest, err, messages...)
}

func UnauthorizedError(err error, messages ...string) error {
	return WithStatus(http.StatusUnauthorized, err, messages...)
}

func ForbiddenError(err error, messages ...string) error {
	return WithStatus(http.StatusForbidden, err, messages...)
}

func NotFoundError(err error, messages ...string) error {
	return WithStatus(http.StatusNotFound, err, messages...)
}

func InternalServerError(err error, messages ...string) error {
	return WithStatus(http.StatusInternalServerError, err, messages...)
}

func httpError(status int32) error {
	msg := http.StatusText(int(status))
	if msg == "" {
		msg = "Unknown error"
	}
	return errors.New(msg)
}
