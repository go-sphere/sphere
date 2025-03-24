package authorizer

import (
	"context"
)

type authError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e authError) Error() string {
	return e.Message
}

func (e authError) Status() int {
	return e.Code
}

var (
	NeedLoginError  = authError{Code: 401, Message: "need login"}
	PermissionError = authError{Code: 403, Message: "permission denied"}
)

type ContextUtils[I UID] struct{}

func (ContextUtils[I]) GetCurrentID(ctx context.Context) (I, error) {
	var zeroID I
	raw := ctx.Value(ContextKeyUID)
	if raw == nil {
		return zeroID, NeedLoginError
	}
	id, ok := raw.(I)
	if !ok {
		return zeroID, NeedLoginError
	}
	return id, nil
}

func (ContextUtils[I]) GetCurrentUsername(ctx context.Context) (string, error) {
	raw := ctx.Value(ContextKeySubject)
	username, ok := raw.(string)
	if !ok {
		return "", NeedLoginError
	}
	return username, nil
}

func (c ContextUtils[I]) CheckAuthStatus(ctx context.Context) error {
	_, err := c.GetCurrentID(ctx)
	return err
}

func (c ContextUtils[I]) CheckAuthID(ctx context.Context, id I) error {
	currentId, err := c.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}
