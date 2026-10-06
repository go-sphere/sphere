package jwtauth_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/auth/authorizer"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
	"github.com/go-sphere/sphere/server/httpz"
	authmw "github.com/go-sphere/sphere/server/middleware/auth"
	"github.com/golang-jwt/jwt/v5"
)

func ExampleNewJwtAuth() {
	ctx := context.Background()
	tokens := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]]("change-me")

	claims := jwtauth.NewRBACClaims(int64(42), "ada", []string{"admin"}, time.Now().Add(time.Hour))
	token, err := tokens.GenerateToken(ctx, claims)
	if err != nil {
		fmt.Println("generate:", err)
		return
	}

	parsed, err := tokens.ParseToken(ctx, token)
	if err != nil {
		fmt.Println("parse:", err)
		return
	}
	uid, err := parsed.GetUID()
	fmt.Println(uid, err, parsed.Subject, parsed.Roles)
	// Output:
	// 42 <nil> ada [admin]
}

func ExampleJwtAuth_ParseToken_rejected() {
	ctx := context.Background()
	tokens := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]]("change-me")

	expired, err := tokens.GenerateToken(ctx,
		jwtauth.NewRBACClaims(int64(42), "ada", nil, time.Now().Add(-time.Minute)))
	if err != nil {
		fmt.Println("generate:", err)
		return
	}
	_, err = tokens.ParseToken(ctx, expired)
	fmt.Println(errors.Is(err, jwt.ErrTokenExpired))

	other := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]]("another-secret")
	forged, err := other.GenerateToken(ctx,
		jwtauth.NewRBACClaims(int64(42), "ada", nil, time.Now().Add(time.Hour)))
	if err != nil {
		fmt.Println("generate:", err)
		return
	}
	_, err = tokens.ParseToken(ctx, forged)
	fmt.Println(errors.Is(err, jwt.ErrTokenSignatureInvalid))
	// Output:
	// true
	// true
}

func ExampleRBACClaims_GetUID_missing() {
	// A token without a uid (for example one minted for another purpose with the
	// same secret) must never authenticate as user 0.
	_, err := jwtauth.RBACClaims[int64]{}.GetUID()
	fmt.Println(errors.Is(err, authorizer.MissingUIDError))
	// Output:
	// true
}

// This example authenticates requests with a bearer JWT and reads the
// identity in the handler.
func Example_middleware() {
	tokens := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]]("change-me")

	engine := stdx.New(stdx.WithErrorHandler(httpz.AbortWithJsonError))
	api := engine.Group("/api", authmw.NewAuthMiddleware[int64, jwtauth.RBACClaims[int64]](tokens,
		authmw.WithPrefixTransform(authmw.AuthorizationPrefixBearer)))
	api.GET("/me", httpz.WithJson(func(ctx httpx.Context) (int64, error) {
		return authorizer.ContextUtils[int64]{}.GetCurrentID(ctx.Context())
	}))

	token, err := tokens.GenerateToken(context.Background(),
		jwtauth.NewRBACClaims(int64(42), "ada", nil, time.Now().Add(time.Hour)))
	if err != nil {
		fmt.Println("generate:", err)
		return
	}

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	for _, header := range []string{"Bearer " + token, ""} {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		resp, err := tr.Do(req)
		if err != nil {
			fmt.Println("request:", err)
			return
		}
		_ = resp.Body.Close()
		fmt.Println(resp.StatusCode)
	}
	// Output:
	// 200
	// 401
}
