package auth

import (
	"context"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
)

const (
	ContextKeyUID       = "uid"
	ContextKeySubject   = "subject"
	ContextKeyRoles     = "roles"
	AuthorizationHeader = "Authorization"
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
	NeedLoginError  = Error{401, "need login"}
	PermissionError = Error{403, "permission denied"}
)

type Auth[I authorizer.UID] struct {
	zeroID I
	prefix string
	parser authorizer.Parser[I]
}

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func NewAuth[I authorizer.UID](prefix string, parser authorizer.Parser[I]) *Auth[I] {
	return &Auth[I]{
		prefix: prefix,
		parser: parser,
	}
}

func (a *Auth[I]) GetCurrentID(ctx context.Context) (I, error) {
	raw := ctx.Value(ContextKeyUID)
	if raw == nil {
		return a.zeroID, NeedLoginError
	}
	id, ok := raw.(I)
	if !ok {
		return a.zeroID, NeedLoginError
	}
	return id, nil
}

func (a *Auth[I]) GetCurrentUsername(ctx context.Context) (string, error) {
	raw := ctx.Value(ContextKeySubject)
	username, ok := raw.(string)
	if !ok {
		return "", NeedLoginError
	}
	return username, nil
}

func (a *Auth[I]) CheckAuthStatus(ctx context.Context) error {
	_, err := a.GetCurrentID(ctx)
	return err
}

func (a *Auth[I]) CheckAuthID(ctx context.Context, id I) error {
	currentId, err := a.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}
