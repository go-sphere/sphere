package auth

import (
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/auth/authorizer"
)

var (
	errPermissionDenied = httpx.NewForbiddenError("no permission to access this resource")
)

// AccessControl defines the interface for checking access permissions.
// Implementations should determine if a given role has access to a specific resource.
type AccessControl interface {
	IsAllowed(role, resource string) bool
}

// NewPermissionMiddleware creates a role-based access control middleware.
// It checks if any of the user's roles have permission to access the specified resource
// using the provided AccessControl implementation.
func NewPermissionMiddleware[I authorizer.UID](resource string, acl AccessControl) httpx.Middleware {
	return func(ctx httpx.Context) error {
		isAllowed := false
		authData, exist := authorizer.GetAuthData[I](ctx.Context())
		if !exist {
			return errPermissionDenied
		}
		for _, r := range authData.Roles {
			if acl.IsAllowed(r, resource) {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return errPermissionDenied
		}
		return ctx.Next()
	}
}
