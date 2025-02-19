package ginx

import (
	"errors"
	"net/http"
)

var internalServerError = NewError(http.StatusInternalServerError, 500, "Internal Server Error")

type statusError interface {
	error
	Status() int
}

type codeError interface {
	error
	Code() int
}

type Error struct {
	status  int
	code    int
	message string
}

func NewError(status, code int, message string) error {
	return &Error{
		status:  status,
		code:    code,
		message: message,
	}
}

func (e *Error) Error() string {
	return e.message
}

func (e *Error) Status() int {
	return e.status
}

func (e *Error) Code() int {
	return e.code
}

func parseError(err error) (code int, status int, message string) {
	var se statusError
	if errors.As(err, &se) {
		status = se.Status()
	} else {
		status = http.StatusInternalServerError
	}
	var ce codeError
	if errors.As(err, &ce) {
		code = ce.Code()
	} else {
		code = 500
	}
	message = err.Error()
	return
}
