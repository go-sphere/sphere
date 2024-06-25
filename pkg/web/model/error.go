package model

import "errors"

type HTTPError struct {
	Code    int
	Message string
}

func NewHTTPError(code int, message string) HTTPError {
	return HTTPError{
		Code:    code,
		Message: message,
	}
}

func JoinHTTPError(code int, err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(err, NewHTTPError(code, err.Error()))
}

func (e HTTPError) Error() string {
	return e.Message
}

var (
	NeedLoginError  = NewHTTPError(401, "需要登录")
	PermissionError = NewHTTPError(403, "权限不足")
)
