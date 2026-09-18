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

// NewPermissionMiddleware checks whether any authenticated role is allowed for resource.
// Missing auth data or no matching role is denied with httpx.NewForbiddenError, not authorizer.PermissionError.
func NewPermissionMiddleware[I authorizer.UID](resource string, acl AccessControl) httpx.Middleware {
	return httpx.AsMiddleware(NewPermissionInterceptor[I](resource, acl))
}

// NewPermissionInterceptor is NewPermissionMiddleware as an httpx.Interceptor.
func NewPermissionInterceptor[I authorizer.UID](resource string, acl AccessControl) httpx.Interceptor {
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
