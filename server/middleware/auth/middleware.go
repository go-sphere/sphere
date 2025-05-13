package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/gin-gonic/gin"
)

var (
	errTokenNotFound = errors.New("token not found")
)

const (
	AuthorizationHeader = "Authorization"
)

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func abortUnauthorized(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"message": "unauthorized",
	})
}

func abortForbidden(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"message": "forbidden",
	})
}

func parserToken[T authorizer.UID](ctx *gin.Context, raw string, loader func(text string) (string, error), parser authorizer.Parser[authorizer.RBACClaims[T]]) error {
	if raw == "" {
		return errTokenNotFound
	}
	token, err := loader(raw)
	if err != nil {
		return err
	}
	if token == "" {
		return errTokenNotFound
	}
	claims, err := parser.ParseToken(token)
	if err != nil {
		return err
	}
	ctx.Set(authorizer.ContextKeyUID, claims.UID)
	ctx.Set(authorizer.ContextKeySubject, claims.Subject)
	ctx.Set(authorizer.ContextKeyRoles, claims.Roles)
	return nil
}

func NewAuthMiddleware[T authorizer.UID](prefix string, parser authorizer.Parser[authorizer.RBACClaims[T]], abortOnError bool) gin.HandlerFunc {
	prefix = strings.TrimSpace(prefix)
	if len(prefix) > 0 {
		prefix = prefix + " "
	}
	return func(ctx *gin.Context) {
		token := ctx.GetHeader(AuthorizationHeader)
		err := parserToken(ctx, token, func(text string) (string, error) {
			if len(prefix) > 0 && strings.HasPrefix(token, prefix) {
				token = strings.TrimSpace(strings.TrimPrefix(token, prefix))
			}
			return token, nil
		}, parser)
		if err != nil && abortOnError {
			abortUnauthorized(ctx)
			return
		}
		ctx.Next()
	}
}

func NewCookieAuthMiddleware[T authorizer.UID](cookieName string, loader func(raw string) (string, error), parser authorizer.Parser[authorizer.RBACClaims[T]], abortOnError bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie(cookieName)
		if err != nil && abortOnError {
			abortUnauthorized(ctx)
			return
		}
		err = parserToken(ctx, token, loader, parser)
		if err != nil && abortOnError {
			abortUnauthorized(ctx)
			return
		}
		ctx.Next()
	}
}

func NewPermissionMiddleware(resource string, acl AccessControl) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		isAllowed := false
		if rolesRaw, exist := ctx.Get(authorizer.ContextKeyRoles); exist {
			if roles, ok := rolesRaw.([]string); ok {
				for _, r := range roles {
					if acl.IsAllowed(r, resource) {
						isAllowed = true
						break
					}
				}
			}
		}
		if isAllowed {
			ctx.Next()
		} else {
			abortForbidden(ctx)
			return
		}
	}
}
