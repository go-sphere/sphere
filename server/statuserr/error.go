package statuserr

import (
	"errors"
	"net/http"
)

type Error struct {
	status  int
	code    int
	Message string
}

func NewError(status, code int, message string) Error {
	return Error{
		status:  status,
		code:    code,
		Message: message,
	}
}

func JoinError(status int, err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(err, NewError(status, 0, ""))
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Status() int {
	return e.status
}

func (e Error) Code() int {
	return e.code
}

func ForbiddenError(message string) error {
	return NewError(http.StatusForbidden, 0, message)
}

func NotFoundError(message string) error {
	return NewError(http.StatusNotFound, 0, message)
}

func InternalServerError(message string) error {
	return NewError(http.StatusInternalServerError, 0, message)
}

func BadRequestError(message string) error {
	return NewError(http.StatusBadRequest, 0, message)
}

func JoinForbiddenError(err error) error {
	return JoinError(http.StatusForbidden, err)
}

func JoinNotFoundError(err error) error {
	return JoinError(http.StatusNotFound, err)
}

func JoinInternalServerError(err error) error {
	return JoinError(http.StatusInternalServerError, err)
}

func JoinBadRequestError(err error) error {
	return JoinError(http.StatusBadRequest, err)
}
