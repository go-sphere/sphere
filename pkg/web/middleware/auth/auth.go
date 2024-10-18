package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/sphere/pkg/web/auth/authorizer"
	"net/http"
	"strconv"
	"strings"
)

type Auth struct {
	*Base[int64, string]
	prefix string
	parser authorizer.Parser
}

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func NewAuth(prefix string, parser authorizer.Parser) *Auth {
	return &Auth{
		Base:   &Base[int64, string]{},
		prefix: prefix,
		parser: parser,
	}
}

func (w *Auth) NewAuthMiddleware(abortOnError bool) gin.HandlerFunc {
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

		if len(w.prefix) > 0 && strings.HasPrefix(token, w.prefix+" ") {
			token = token[len(w.prefix)+1:]
		}

		claims, err := w.parser.ParseToken(token)
		if err != nil {
			abort(ctx)
			return
		}

		id, _ := strconv.Atoi(claims.Subject)
		ctx.Set(ContextKeyID, int64(id))
		ctx.Set(ContextKeyUsername, claims.Username)
		ctx.Set(ContextKeyRoles, claims.Username)
	}
}

func (w *Auth) NewPermissionMiddleware(resource string, acl AccessControl) gin.HandlerFunc {
	abort := func(ctx *gin.Context) {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "forbidden",
		})
	}
	return func(ctx *gin.Context) {
		rolesRaw, exist := ctx.Get(ContextKeyRoles)
		if !exist {
			abort(ctx)
			return
		}

		roleStr, ok := rolesRaw.(string)
		if !ok {
			abort(ctx)
			return
		}

		roles := w.parser.ParseRoles(roleStr)
		for _, r := range roles {
			if acl.IsAllowed(r, resource) {
				return
			}
		}

		abort(ctx)
	}
}
