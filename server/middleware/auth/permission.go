package auth

import (
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/auth/authorizer"
)

// AccessControl defines the interface for checking access permissions.
// Implementations should determine if a given role has access to a specific resource.
type AccessControl interface {
	IsAllowed(role, resource string) bool
}

// NewPermissionMiddleware creates a role-based access control middleware.
// It checks if any of the user's roles have permission to access the specified resource
// using the provided AccessControl implementation.
func NewPermissionMiddleware(resource string, acl AccessControl) httpx.Middleware {
	return func(ctx httpx.Context) error {
		isAllowed := false
		for _, r := range authorizer.GetCurrentRoles(ctx) {
			if acl.IsAllowed(r, resource) {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return httpx.NewForbiddenError("no permission to access this resource")
		}
		return ctx.Next()
	}
}
