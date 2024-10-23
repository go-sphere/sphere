package auth

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

func (a *Auth[I, U]) NewAuthMiddleware(abortOnError bool) gin.HandlerFunc {
	abort := func(ctx *gin.Context) {
		if abortOnError {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "unauthorized",
			})
		}
	}

	bitMap := map[reflect.Kind]int{
		reflect.Int:    strconv.IntSize,
		reflect.Int8:   8,
		reflect.Int16:  16,
		reflect.Int32:  32,
		reflect.Int64:  64,
		reflect.Uint:   64,
		reflect.Uint8:  strconv.IntSize,
		reflect.Uint16: 16,
		reflect.Uint32: 32,
		reflect.Uint64: 64,
	}

	idSetter := func(t reflect.Type) func(ctx *gin.Context, id string) {
		switch t.Kind() {
		case reflect.String:
			return func(ctx *gin.Context, id string) {
				ctx.Set(ContextKeyID, id)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			bit := bitMap[t.Kind()]
			return func(ctx *gin.Context, id string) {
				num, err := strconv.ParseInt(id, 10, bit)
				if err != nil {
					abort(ctx)
					return
				}
				ctx.Set(ContextKeyID, I(num))
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			bit := bitMap[t.Kind()]
			return func(ctx *gin.Context, id string) {
				num, err := strconv.ParseUint(id, 10, bit)
				if err != nil {
					abort(ctx)
					return
				}
				ctx.Set(ContextKeyID, I(num))
			}
		default:
			return func(ctx *gin.Context, id string) {
				abort(ctx)
			}
		}
	}(reflect.TypeOf(a.zeroID))

	return func(ctx *gin.Context) {
		token := ctx.GetHeader(AuthorizationHeader)
		if token == "" {
			abort(ctx)
			return
		}

		if len(a.prefix) > 0 && strings.HasPrefix(token, a.prefix+" ") {
			token = token[len(a.prefix)+1:]
		}

		claims, err := a.parser.ParseToken(token)
		if err != nil {
			abort(ctx)
			return
		}
		idSetter(ctx, claims.Subject)
		ctx.Set(ContextKeyUsername, claims.Username)
		ctx.Set(ContextKeyRoles, claims.Roles)
	}
}

func (a *Auth[I, U]) NewPermissionMiddleware(resource string, acl AccessControl) gin.HandlerFunc {
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

		roles := a.parser.ParseRoles(roleStr)
		for _, r := range roles {
			if acl.IsAllowed(r, resource) {
				return
			}
		}

		abort(ctx)
	}
}
