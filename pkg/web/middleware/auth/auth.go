package auth

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

type Auth struct {
	*Context
	authPrefix string
	validator  Validator
}

func NewAuth(authPrefix string, validators Validator) *Auth {
	return &Auth{
		Context:    &Context{},
		authPrefix: authPrefix,
		validator:  validators,
	}
}

func (w *Auth) NewAuthMiddleware(abortOnError bool) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader(AuthorizationHeader)
		abort := func() {
			if abortOnError {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"message": "unauthorized",
				})
			}
		}
		if token == "" {
			abort()
			return
		}
		if len(w.authPrefix) > 0 && strings.HasPrefix(token, w.authPrefix+" ") {
			token = token[len(w.authPrefix)+1:]
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

func (w *Auth) NewPermissionMiddleware(per string) func(context *gin.Context) {
	return func(ctx *gin.Context) {
		err := w.CheckAuthPermission(ctx, per)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": err.Error(),
			})
		}
	}
}
