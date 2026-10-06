// Package jwtauth signs and verifies HMAC JSON Web Tokens. [JwtAuth]
// satisfies authorizer.Parser and authorizer.Generator when its claims type is
// both a jwt.Claims and an authorizer.Claims, such as [RBACClaims].
//
// Build one [JwtAuth] per secret with [NewJwtAuth], issue tokens with
// [JwtAuth.GenerateToken], and pass the same value to
// server/middleware/auth.NewAuthMiddleware to authenticate requests.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/server/auth/jwtauth"
//		authmw "github.com/go-sphere/sphere/server/middleware/auth"
//	)
//
//	tokens := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]](conf.JWTSecret)
//
//	claims := jwtauth.NewRBACClaims(int64(42), "ada", []string{"admin"}, time.Now().Add(time.Hour))
//	token, err := tokens.GenerateToken(ctx, claims)
//	if err != nil {
//		return err
//	}
//
//	router.Use(authmw.NewAuthMiddleware[int64, jwtauth.RBACClaims[int64]](tokens,
//		authmw.WithPrefixTransform(authmw.AuthorizationPrefixBearer)))
//
// The default algorithm is HS256; [WithSigningMethod] selects another HMAC
// method. [NewJwtAuth] panics on an empty secret. A [RBACClaims] with a zero
// UID is rejected with authorizer.MissingUIDError, so a token signed with the
// same secret for another purpose cannot authenticate as user 0.
package jwtauth
