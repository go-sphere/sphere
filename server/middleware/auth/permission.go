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
// IsAllowed is called on the request path for each role of the current user, so
// it must be safe for concurrent use and should deny unknown roles or resources.
// server/auth/acl.ACL implements it.
type AccessControl interface {
	IsAllowed(role, resource string) bool
}

// NewPermissionMiddleware checks whether any authenticated role is allowed for resource.
// Missing auth data or no matching role is denied with httpx.NewForbiddenError, not authorizer.PermissionError.
//
// Register it after NewAuthMiddleware so auth data is present. I must be the UID
// type the auth middleware stored; with a different type the data is invisible
// and every request is denied with 403.
func NewPermissionMiddleware[I authorizer.UID](resource string, acl AccessControl) httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			authData, exist := authorizer.GetAuthData[I](ctx.Context())
			if !exist {
				return errPermissionDenied
			}
			for _, r := range authData.Roles {
				if acl.IsAllowed(r, resource) {
					return next(ctx)
				}
			}
			return errPermissionDenied
		}
	}
}
