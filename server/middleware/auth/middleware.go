package auth

import (
	"net/http"
	"strings"

	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/gin-gonic/gin"
)

const (
	AuthorizationHeader = "Authorization"
)

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func abort(ctx *gin.Context, abortOnError bool) {
	if abortOnError {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "unauthorized",
		})
	}
}

func parserToken(ctx *gin.Context, raw string, loader func(text string) (string, error), parser authorizer.Parser[authorizer.RBACClaims[int64]], abortOnError bool) {
	token, err := loader(raw)
	if err != nil {
		abort(ctx, abortOnError)
		return
	}
	claims, err := parser.ParseToken(token)
	if err != nil {
		abort(ctx, abortOnError)
		return
	}
	ctx.Set(authorizer.ContextKeyUID, claims.UID)
	ctx.Set(authorizer.ContextKeySubject, claims.Subject)
	ctx.Set(authorizer.ContextKeyRoles, claims.Roles)
}

func NewAuthMiddleware(prefix string, parser authorizer.Parser[authorizer.RBACClaims[int64]], abortOnError bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader(AuthorizationHeader)
		if token == "" {
			abort(ctx, abortOnError)
			return
		}
		parserToken(ctx, token, func(text string) (string, error) {
			if len(prefix) > 0 && strings.HasPrefix(token, prefix+" ") {
				token = strings.TrimSpace(strings.TrimPrefix(token, prefix+" "))
			}
			return token, nil
		}, parser, abortOnError)
	}
}

func NewCookieAuthMiddleware(cookieName string, loader func(raw string) (string, error), parser authorizer.Parser[authorizer.RBACClaims[int64]], abortOnError bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie(cookieName)
		if err != nil {
			abort(ctx, abortOnError)
			return
		}
		parserToken(ctx, token, loader, parser, abortOnError)
	}
}

func NewPermissionMiddleware(resource string, acl AccessControl) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		rolesRaw, exist := ctx.Get(authorizer.ContextKeyRoles)
		if !exist {
			abort(ctx, true)
			return
		}

		roles, ok := rolesRaw.([]string)
		if !ok {
			abort(ctx, true)
			return
		}

		for _, r := range roles {
			if acl.IsAllowed(r, resource) {
				return
			}
		}
		abort(ctx, true)
	}
}
