package auth

import (
	"context"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"golang.org/x/exp/constraints"
)

const (
	ContextKeyID        = "uid"
	ContextKeyUsername  = "username"
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

type ID interface {
	constraints.Integer | string
}

type UserName interface {
	string
}

type Auth[I ID, U UserName] struct {
	zeroID   I
	zeroName U
	prefix   string
	parser   authorizer.Parser
}

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func NewAuth[I ID, U UserName](prefix string, parser authorizer.Parser) *Auth[I, U] {
	return &Auth[I, U]{
		prefix: prefix,
		parser: parser,
	}
}

func (a *Auth[I, U]) GetCurrentID(ctx context.Context) (I, error) {
	raw := ctx.Value(ContextKeyID)
	if raw == nil {
		return a.zeroID, NeedLoginError
	}
	id, ok := raw.(I)
	if !ok {
		return a.zeroID, NeedLoginError
	}
	return id, nil
}

func (a *Auth[I, U]) GetCurrentUsername(ctx context.Context) (U, error) {
	raw := ctx.Value(ContextKeyUsername)
	if raw == nil {
		return a.zeroName, NeedLoginError
	}
	username, ok := raw.(U)
	if !ok {
		return a.zeroName, NeedLoginError
	}
	return username, nil
}

func (a *Auth[I, U]) CheckAuthStatus(ctx context.Context) error {
	_, err := a.GetCurrentID(ctx)
	return err
}

func (a *Auth[I, U]) CheckAuthID(ctx context.Context, id I) error {
	currentId, err := a.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}
