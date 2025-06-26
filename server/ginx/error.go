package ginx

import (
	"errors"
	"net/http"

	"github.com/TBXark/sphere/core/errors/statuserr"
)

func ParseError(err error) (code int32, status int32, message string) {
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
