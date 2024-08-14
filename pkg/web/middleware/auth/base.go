package auth

import "github.com/gin-gonic/gin"

const (
	ContextKeyID        = "uid"
	ContextKeyUsername  = "username"
	ContextKeyRoles     = "roles"
	AuthorizationHeader = "Authorization"
	AllPermissionRole   = "all"
)

type Validator interface {
	Validate(token string) (map[string]any, error)
	ParseRolesString(roles string) map[string]struct{}
}

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

func (b *Base) CheckAuthPermission(ctx *gin.Context, permission string) error {
	permissionList, exist := ctx.Get(ContextKeyRoles)
	if !exist {
		return PermissionError
	}
	permissions := permissionList.(map[string]struct{})
	if _, o := permissions[AllPermissionRole]; o {
		return nil
	}
	if _, o := permissions[permission]; o {
		return nil
	}
	return PermissionError
}
