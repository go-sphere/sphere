package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

const (
	ContextKeyID        = "uid"
	ContextKeyUsername  = "username"
	ContextKeyRoles     = "roles"
	AuthorizationHeader = "Authorization"
	AllPermissionRole   = "all"
)

type JwtValidator interface {
	Validate(token string) (map[string]any, error)
	ParseRolesString(roles string) map[string]struct{}
}

type JwtAuth struct {
	validator JwtValidator
}

func NewJwtAuth(validators JwtValidator) *JwtAuth {
	return &JwtAuth{
		validator: validators,
	}
}

type jwtError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (j jwtError) Error() string {
	return j.Message
}

func (j jwtError) Status() int {
	return j.Code
}

var (
	NeedLoginError  = jwtError{401, "need login"}
	PermissionError = jwtError{403, "permission denied"}
)

func (w *JwtAuth) GetCurrentID(ctx *gin.Context) (int, error) {
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

func (w *JwtAuth) CheckAuthStatus(ctx *gin.Context) error {
	_, err := w.GetCurrentID(ctx)
	return err
}

func (w *JwtAuth) CheckAuthID(ctx *gin.Context, id int) error {
	currentId, err := w.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}

func (w *JwtAuth) CheckAuthPermission(ctx *gin.Context, permission string) error {
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

func (w *JwtAuth) NewJwtAuthMiddleware(abortOnError bool) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader(AuthorizationHeader)
		abort := func() {
			if abortOnError {
				ctx.JSON(http.StatusUnauthorized, gin.H{
					"message": "unauthorized",
				})
				ctx.Abort()
			}
		}
		if token == "" {
			abort()
			return
		}
		claims, err := w.validator.Validate(token)
		if err != nil {
			abort()
			return
		}
		stringLoader := func(key string) string {
			v, e := claims[key]
			if !e {
				return ""
			}
			s, e := v.(string)
			if !e {
				return ""
			}
			return s
		}
		id, err := strconv.Atoi(stringLoader(ContextKeyID))
		if err != nil {
			abort()
			return
		}
		ctx.Set(ContextKeyID, id)
		ctx.Set(ContextKeyUsername, stringLoader(ContextKeyUsername))
		ctx.Set(ContextKeyRoles, w.validator.ParseRolesString(stringLoader(ContextKeyRoles)))
	}
}

func (w *JwtAuth) JwtAuthMiddleware(ctx *gin.Context) {
	w.NewJwtAuthMiddleware(true)(ctx)
}

func (w *JwtAuth) NewPermissionMiddleware(per string) func(context *gin.Context) {
	return func(context *gin.Context) {
		err := w.CheckAuthPermission(context, per)
		if err != nil {
			context.JSON(http.StatusForbidden, gin.H{
				"message": err.Error(),
			})
			context.Abort()
		}
	}
}
