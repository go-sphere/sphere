package auth

import "github.com/gin-gonic/gin"

const (
	ContextKeyID        = "uid"
	ContextKeyUsername  = "username"
	ContextKeyRoles     = "roles"
	AuthorizationHeader = "Authorization"
)

type Context struct {
}

func (c *Context) GetCurrentID(ctx *gin.Context) (int, error) {
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

func (c *Context) GetCurrentUsername(ctx *gin.Context) (string, error) {
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

func (c *Context) CheckAuthStatus(ctx *gin.Context) error {
	_, err := c.GetCurrentID(ctx)
	return err
}

func (c *Context) CheckAuthID(ctx *gin.Context, id int) error {
	currentId, err := c.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}
