// Package auth provides httpx middleware that authenticates requests with an
// authorizer.Parser and authorizes them by role.
//
// [NewAuthMiddleware] loads a token (by default the Authorization header),
// parses it, and stores authorizer.Data on the request context, where handlers
// read it with authorizer.ContextUtils. [NewPermissionMiddleware] then allows
// the request only when one of the stored roles may access a resource
// according to an [AccessControl] such as server/auth/acl.ACL. The middleware
// is parser-agnostic; server/auth/jwtauth is the JWT implementation.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/httpx/stdx"
//		"github.com/go-sphere/sphere/server/auth/acl"
//		"github.com/go-sphere/sphere/server/auth/jwtauth"
//		"github.com/go-sphere/sphere/server/httpz"
//		"github.com/go-sphere/sphere/server/middleware/auth"
//	)
//
//	tokens := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]](conf.JWTSecret)
//	permissions := acl.NewACL()
//	permissions.Allow("admin", "admin-api")
//
//	engine := stdx.New(stdx.WithErrorHandler(httpz.AbortWithJsonError))
//	api := engine.Group("/api", auth.NewAuthMiddleware[int64, jwtauth.RBACClaims[int64]](tokens,
//		auth.WithPrefixTransform(auth.AuthorizationPrefixBearer)))
//	admin := api.Group("/admin", auth.NewPermissionMiddleware[int64]("admin-api", permissions))
//
// Both middlewares reject by returning an error, which the engine's error
// handler renders; installing httpz.AbortWithJsonError keeps the standard
// envelope. Authentication failures are 401; permission failures are 403
// with the English message "no permission to access this resource".
//
// # Defaults
//
//   - The token is read from the Authorization header unchanged. The "Bearer "
//     prefix is stripped only when [WithPrefixTransform] is configured, and a
//     token without the prefix is still parsed as-is.
//   - Failures abort the request; WithAbortOnError(false) lets anonymous
//     requests continue without auth data.
//   - Use server/middleware/selector to apply the middleware only to selected
//     routes.
package auth
