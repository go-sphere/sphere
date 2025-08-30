package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-sphere/sphere/server/auth/authorizer"
)

// AccessControl defines the interface for checking access permissions.
// Implementations should determine if a given role has access to a specific resource.
type AccessControl interface {
	IsAllowed(role, resource string) bool
}

type permissionOptions struct {
	abortWithError func(ctx *gin.Context, status int, err error)
}

// PermissionOption is a functional option for configuring permission middleware behavior.
type PermissionOption func(*permissionOptions)

// WithAbortForbidden sets a custom error handler for permission denied scenarios.
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

// NewPermissionMiddleware creates a role-based access control middleware.
// It checks if any of the user's roles have permission to access the specified resource
// using the provided AccessControl implementation.
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
