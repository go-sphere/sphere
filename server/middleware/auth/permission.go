package auth

import (
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/gin-gonic/gin"
	"net/http"
)

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

func abortForbidden(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error":   "forbidden",
		"message": "没有权限访问该资源",
	})
}

func NewPermissionMiddleware(resource string, acl AccessControl) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		isAllowed := false
		for _, r := range authorizer.GetCurrentRoles(ctx) {
			if acl.IsAllowed(r, resource) {
				isAllowed = true
				break
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
