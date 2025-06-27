package ginx

import (
	"errors"
	"net/http"

	"github.com/TBXark/sphere/core/errors/statuserr"
	"github.com/gin-gonic/gin"
)

type ErrorParser func(error) (int32, int32, string)

var defaultErrorParser ErrorParser = ParseError

func SetDefaultErrorParser(parser ErrorParser) {
	defaultErrorParser = parser
}

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

func AbortWithJsonError(ctx *gin.Context, err error) {
	code, status, message := defaultErrorParser(err)
	if status < 100 || status > 599 {
		status = http.StatusInternalServerError
	}
	ctx.AbortWithStatusJSON(int(status), ErrorResponse{
		Code:    int(code),
		Error:   err.Error(),
		Message: message,
	})
}
