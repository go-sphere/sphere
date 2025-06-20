package authorizer

import (
	"context"
)

const (
	ContextKeyUID     = "uid"
	ContextKeySubject = "subject"
	ContextKeyRoles   = "roles"
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

func GetCurrentSubject(ctx context.Context) (string, error) {
	raw := ctx.Value(ContextKeySubject)
	username, ok := raw.(string)
	if !ok {
		return "", NeedLoginError
	}
	return username, nil
}

func GetCurrentRoles(ctx context.Context) []string {
	raw := ctx.Value(ContextKeyRoles)
	if raw == nil {
		return nil
	}
	roles, ok := raw.([]string)
	if !ok {
		return nil
	}
	return roles
}
