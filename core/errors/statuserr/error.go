package statuserr

import (
	"errors"
	"net/http"
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
		return nil
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
		return nil
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

func BadRequestError(err error, message string) error {
	return JoinError(http.StatusBadRequest, message, err)
}

func UnauthorizedError(err error, message string) error {
	return JoinError(http.StatusUnauthorized, message, err)
}

func ForbiddenError(err error, message string) error {
	return JoinError(http.StatusForbidden, message, err)
}

func NotFoundError(err error, message string) error {
	return JoinError(http.StatusNotFound, message, err)
}

func InternalServerError(err error, message string) error {
	return JoinError(http.StatusInternalServerError, message, err)
}
