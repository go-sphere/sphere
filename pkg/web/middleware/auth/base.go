package auth

import (
	"context"
)

const (
	ContextKeyID        = "uid"
	ContextKeyUsername  = "username"
	ContextKeyRoles     = "roles"
	AuthorizationHeader = "Authorization"
)

type Base[ID comparable, USERNAME any] struct {
	zeroID   ID
	zeroName USERNAME
}

func (b *Base[ID, USERNAME]) GetCurrentID(ctx context.Context) (ID, error) {
	raw := ctx.Value(ContextKeyID)
	if raw == nil {
		return b.zeroID, NeedLoginError
	}
	id, ok := raw.(ID)
	if !ok {
		return b.zeroID, NeedLoginError
	}
	return id, nil
}

func (b *Base[ID, USERNAME]) GetCurrentUsername(ctx context.Context) (USERNAME, error) {
	raw := ctx.Value(ContextKeyUsername)
	if raw == nil {
		return b.zeroName, NeedLoginError
	}
	username, ok := raw.(USERNAME)
	if !ok {
		return b.zeroName, NeedLoginError
	}
	return username, nil
}

func (b *Base[ID, USERNAME]) CheckAuthStatus(ctx context.Context) error {
	_, err := b.GetCurrentID(ctx)
	return err
}

func (b *Base[ID, USERNAME]) CheckAuthID(ctx context.Context, id ID) error {
	currentId, err := b.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}
