package ginx

import (
	"errors"
	"github.com/TBXark/sphere/server/statuserr"
	"net/http"
)

var internalServerError = statuserr.NewError(http.StatusInternalServerError, 0, "Internal Server Error")

type statusError interface {
	error
	Status() int
}

type codeError interface {
	error
	Code() int
}

func ParseError(err error) (code int, status int, message string) {
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
		code = status
	}
	message = err.Error()
	return
}
