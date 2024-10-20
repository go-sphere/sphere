package auth

import (
	"context"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
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

type Auth[ID comparable, USERNAME comparable] struct {
	zeroID   ID
	zeroName USERNAME
	prefix   string
	parser   authorizer.Parser
}

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func NewAuth[ID comparable, USERNAME comparable](prefix string, parser authorizer.Parser) *Auth[ID, USERNAME] {
	return &Auth[ID, USERNAME]{
		prefix: prefix,
		parser: parser,
	}
}

func (a *Auth[ID, USERNAME]) GetCurrentID(ctx context.Context) (ID, error) {
	raw := ctx.Value(ContextKeyID)
	if raw == nil {
		return a.zeroID, NeedLoginError
	}
	id, ok := raw.(ID)
	if !ok {
		return a.zeroID, NeedLoginError
	}
	return id, nil
}

func (a *Auth[ID, USERNAME]) GetCurrentUsername(ctx context.Context) (USERNAME, error) {
	raw := ctx.Value(ContextKeyUsername)
	if raw == nil {
		return a.zeroName, NeedLoginError
	}
	username, ok := raw.(USERNAME)
	if !ok {
		return a.zeroName, NeedLoginError
	}
	return username, nil
}

func (a *Auth[ID, USERNAME]) CheckAuthStatus(ctx context.Context) error {
	_, err := a.GetCurrentID(ctx)
	return err
}

func (a *Auth[ID, USERNAME]) CheckAuthID(ctx context.Context, id ID) error {
	currentId, err := a.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}
