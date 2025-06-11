package ginx

import (
	"errors"
	"net/http"

	"github.com/TBXark/sphere/server/statuserr"
)

var internalServerError = statuserr.NewError(http.StatusInternalServerError, 0, "internal Server Error")

func ParseError(err error) (code int, status int, message string) {
	var se statuserr.StatusErr
	if errors.As(err, &se) {
		status = se.Status()
	} else {
		status = http.StatusInternalServerError
	}
	var ce statuserr.CodeErr
	if errors.As(err, &ce) {
		code = ce.Code()
	} else {
		code = status
	}
	var me statuserr.MessageErr
	if errors.As(err, &me) {
		message = me.Message()
	} else {
		message = err.Error()
	}
	return
}
