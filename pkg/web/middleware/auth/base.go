package auth

import "github.com/gin-gonic/gin"

const (
	ContextKeyID        = "uid"
	ContextKeyUsername  = "username"
	ContextKeyRoles     = "roles"
	AuthorizationHeader = "Authorization"
)

type Base struct {
}

func (b *Base) GetCurrentID(ctx *gin.Context) (int, error) {
	raw, exist := ctx.Get(ContextKeyID)
	if !exist {
		return 0, NeedLoginError
	}
	id, ok := raw.(int)
	if !ok {
		return 0, NeedLoginError
	}
	return id, nil
}

func (b *Base) GetCurrentUsername(ctx *gin.Context) (string, error) {
	raw, exist := ctx.Get(ContextKeyUsername)
	if !exist {
		return "", NeedLoginError
	}
	username, ok := raw.(string)
	if !ok {
		return "", NeedLoginError
	}
	return username, nil
}

func (b *Base) CheckAuthStatus(ctx *gin.Context) error {
	_, err := b.GetCurrentID(ctx)
	return err
}

func (b *Base) CheckAuthID(ctx *gin.Context, id int) error {
	currentId, err := b.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}
