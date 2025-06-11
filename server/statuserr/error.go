package statuserr

import (
	"errors"
	"net/http"
)

type StatusErr interface {
	error
	Status() int
}

type CodeErr interface {
	error
	Code() int
}

type MessageErr interface {
	error
	Message() string
}

type Error struct {
	error
	status  int
	code    int
	message string
}

func NewError(status, code int, message string) Error {
	return Error{
		error:   errors.New("status_err:" + message),
		status:  status,
		code:    code,
		message: message,
	}
}

func JoinError(status int, message string, err error) error {
	if err == nil {
		return nil
	}
	return Error{
		error:   err,
		status:  status,
		code:    0,
		message: message,
	}
}

func (e Error) Status() int {
	return e.status
}

func (e Error) Code() int {
	return e.code
}

func (e Error) Message() string {
	return e.message
}

func (e Error) Error() string {
	return e.error.Error()
}

func (e Error) Unwrap() error {
	return e.error
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

func BadRequestError(err error, message string) error {
	return JoinError(http.StatusBadRequest, message, err)
}
