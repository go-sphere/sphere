package statuserr

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

func (e HTTPError) Status() int {
	return e.Code
}
