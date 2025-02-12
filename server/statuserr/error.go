package statuserr

import "errors"

type StatusError struct {
	Code    int
	Message string
}

func NewError(code int, message string) StatusError {
	return StatusError{
		Code:    code,
		Message: message,
	}
}

func JoinError(code int, err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(err, NewError(code, err.Error()))
}

func (e StatusError) Error() string {
	return e.Message
}

func (e StatusError) Status() int {
	return e.Code
}
