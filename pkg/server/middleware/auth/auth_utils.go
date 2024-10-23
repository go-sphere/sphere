package auth

import (
	"context"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
)

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Status() int {
	return e.Code
}

var (
	NeedLoginError  = Error{Code: 401, Message: "need login"}
	PermissionError = Error{Code: 403, Message: "permission denied"}
)

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func GetCurrentID[I authorizer.UID](ctx context.Context) (I, error) {
	var zeroID I
	raw := ctx.Value(authorizer.ContextKeyUID)
	if raw == nil {
		return zeroID, NeedLoginError
	}
	id, ok := raw.(I)
	if !ok {
		return zeroID, NeedLoginError
	}
	return id, nil
}

func GetCurrentUsername(ctx context.Context) (string, error) {
	raw := ctx.Value(authorizer.ContextKeySubject)
	username, ok := raw.(string)
	if !ok {
		return "", NeedLoginError
	}
	return username, nil
}

func CheckAuthStatus[I authorizer.UID](ctx context.Context) error {
	_, err := GetCurrentID[I](ctx)
	return err
}

func CheckAuthID[I authorizer.UID](ctx context.Context, id I) error {
	currentId, err := GetCurrentID[I](ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}
