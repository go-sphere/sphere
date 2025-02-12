package auth

import (
	authorizer2 "github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

const (
	AuthorizationHeader = "Authorization"
)

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func NewAuthMiddleware(prefix string, parser authorizer2.Parser[authorizer2.RBACClaims[int64]], abortOnError bool) gin.HandlerFunc {
	abort := func(ctx *gin.Context) {
		if abortOnError {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "unauthorized",
			})
		}
	}

	return func(ctx *gin.Context) {
		token := ctx.GetHeader(AuthorizationHeader)
		if token == "" {
			abort(ctx)
			return
		}

		if len(prefix) > 0 && strings.HasPrefix(token, prefix+" ") {
			token = token[len(prefix)+1:]
		}

		claims, err := parser.ParseToken(token)
		if err != nil {
			abort(ctx)
			return
		}

		ctx.Set(authorizer2.ContextKeyUID, claims.UID)
		ctx.Set(authorizer2.ContextKeySubject, claims.Subject)
		ctx.Set(authorizer2.ContextKeyRoles, claims.Roles)
	}
}

func NewPermissionMiddleware(resource string, acl AccessControl) gin.HandlerFunc {
	abort := func(ctx *gin.Context) {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "forbidden",
		})
	}
	return func(ctx *gin.Context) {
		rolesRaw, exist := ctx.Get(authorizer2.ContextKeyRoles)
		if !exist {
			abort(ctx)
			return
		}

		roles, ok := rolesRaw.([]string)
		if !ok {
			abort(ctx)
			return
		}

		for _, r := range roles {
			if acl.IsAllowed(r, resource) {
				return
			}
		}
		abort(ctx)
	}
}
