package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

func (w *Auth) validateToken(token string) (uid string, username string, roles string, exp int64, err error) {
	if len(w.authPrefix) > 0 && strings.HasPrefix(token, w.authPrefix+" ") {
		token = token[len(w.authPrefix)+1:]
	}
	return w.validator.Validate(token)
}

func (w *Auth) NewAuthMiddleware(abortOnError bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader(AuthorizationHeader)
		if token == "" {
			w.handleUnauthorized(ctx, abortOnError)
			return
		}

		uid, username, roles, exp, err := w.validateToken(token)
		if err != nil || exp < time.Now().Unix() {
			w.handleUnauthorized(ctx, abortOnError)
			return
		}

		w.setContextValues(ctx, uid, username, roles)
	}
}

func (w *Auth) handleUnauthorized(ctx *gin.Context, abortOnError bool) {
	if abortOnError {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
	}
}

func (w *Auth) setContextValues(ctx *gin.Context, uid, username, roles string) {
	id, _ := strconv.Atoi(uid)
	ctx.Set(ContextKeyID, id)
	ctx.Set(ContextKeyUsername, username)
	ctx.Set(ContextKeyRoles, roles)
}

func (w *Auth) NewPermissionMiddleware(resource string, acl *ACL) func(context *gin.Context) {
	return func(ctx *gin.Context) {
		rolesRaw, exist := ctx.Get(ContextKeyRoles)
		if !exist {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "forbidden",
			})
			return
		}
		roleStr, ok := rolesRaw.(string)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "forbidden",
			})
			return
		}
		roles := w.validator.ParseRoles(roleStr)
		for r := range roles {
			if acl.IsAllowed(r, resource) {
				return
			}
		}
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "forbidden",
		})
	}
}
