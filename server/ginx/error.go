package ginx

import (
	"errors"
	"net/http"

	"github.com/TBXark/sphere/core/errors/statuserr"
)

func ParseError(err error) (code int32, status int32, message string) {
	var se statuserr.StatusError
	if errors.As(err, &se) {
		status = se.GetStatus()
	} else {
		status = http.StatusInternalServerError
	}
	var ce statuserr.CodeError
	if errors.As(err, &ce) {
		code = ce.GetCode()
	} else {
		code = status
	}
	var me statuserr.MessageError
	if errors.As(err, &me) {
		message = me.GetMessage()
	} else {
		message = err.Error()
	}
	return
}
