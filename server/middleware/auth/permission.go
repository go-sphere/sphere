package auth

import (
	"errors"
	"net/http"

	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/gin-gonic/gin"
)

type AccessControl interface {
	IsAllowed(role, resource string) bool
}

type permissionOptions struct {
	abortWithError func(ctx *gin.Context, status int, err error)
}

type PermissionOption func(*permissionOptions)

func WithAbortForbidden(fn func(ctx *gin.Context, status int, err error)) PermissionOption {
	return func(opts *permissionOptions) {
		opts.abortWithError = fn
	}
}

func newPermissionOptions(opts ...PermissionOption) *permissionOptions {
	defaults := &permissionOptions{
		abortWithError: func(ctx *gin.Context, status int, err error) {
			ctx.AbortWithStatusJSON(status, gin.H{
				"error": err.Error(),
			})
		},
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

func NewPermissionMiddleware(resource string, acl AccessControl, options ...PermissionOption) gin.HandlerFunc {
	opts := newPermissionOptions(options...)
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
			opts.abortWithError(ctx, http.StatusForbidden, errors.New("no permission to access this resource"))
			return
		}
	}
}
