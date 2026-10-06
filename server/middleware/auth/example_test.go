package auth_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/auth/acl"
	"github.com/go-sphere/sphere/server/auth/authorizer"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/server/middleware/auth"
)

// This example protects an /api group with JWT authentication and an
// /api/admin group with a role check.
func Example() {
	tokens := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]]("change-me")
	permissions := acl.NewACL()
	permissions.Allow("admin", "admin-api")

	engine := stdx.New(stdx.WithErrorHandler(httpz.AbortWithJsonError))
	api := engine.Group("/api", auth.NewAuthMiddleware[int64, jwtauth.RBACClaims[int64]](tokens,
		auth.WithPrefixTransform(auth.AuthorizationPrefixBearer)))
	api.GET("/me", httpz.WithJson(func(ctx httpx.Context) (int64, error) {
		return authorizer.ContextUtils[int64]{}.GetCurrentID(ctx.Context())
	}))
	admin := api.Group("/admin", auth.NewPermissionMiddleware[int64]("admin-api", permissions))
	admin.GET("/stats", httpz.WithText(func(httpx.Context) (string, error) {
		return "ok", nil
	}))

	issue := func(uid int64, roles ...string) string {
		token, err := tokens.GenerateToken(context.Background(),
			jwtauth.NewRBACClaims(uid, "", roles, time.Now().Add(time.Hour)))
		if err != nil {
			panic(err)
		}
		return "Bearer " + token
	}

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	get := func(path, authorization string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if authorization != "" {
			req.Header.Set(auth.AuthorizationHeader, authorization)
		}
		resp, err := tr.Do(req)
		if err != nil {
			fmt.Println("request:", err)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("read:", err)
			return
		}
		fmt.Println(resp.StatusCode, strings.TrimSpace(string(body)))
	}

	get("/api/me", "")
	get("/api/me", issue(7))
	get("/api/admin/stats", issue(7, "viewer"))
	get("/api/admin/stats", issue(1, "admin"))
	// Output:
	// 401 {"success":false,"code":0,"message":"没有提供有效的认证信息"}
	// 200 {"success":true,"data":7}
	// 403 {"success":false,"code":0,"message":"no permission to access this resource"}
	// 200 ok
}

func ExampleWithAbortOnError() {
	parser := authorizer.ParserFunc[string, apiKeyClaims](func(ctx context.Context, token string) (apiKeyClaims, error) {
		if token != "secret-key" {
			return apiKeyClaims{}, authorizer.TokenNotFoundError
		}
		return apiKeyClaims{uid: "service-a"}, nil
	})

	// Optional authentication: anonymous requests continue without auth data.
	engine := stdx.New()
	engine.Group("", auth.NewAuthMiddleware(parser,
		auth.WithHeaderLoader("X-API-Key"),
		auth.WithAbortOnError(false),
	)).GET("/hello", httpz.WithText(func(ctx httpx.Context) (string, error) {
		uid, err := authorizer.ContextUtils[string]{}.GetCurrentID(ctx.Context())
		if err != nil {
			return "hello, guest", nil
		}
		return "hello, " + uid, nil
	}))

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	for _, key := range []string{"", "secret-key"} {
		req := httptest.NewRequest(http.MethodGet, "/hello", nil)
		if key != "" {
			req.Header.Set("X-API-Key", key)
		}
		resp, err := tr.Do(req)
		if err != nil {
			fmt.Println("request:", err)
			return
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			fmt.Println("read:", err)
			return
		}
		fmt.Println(string(body))
	}
	// Output:
	// hello, guest
	// hello, service-a
}

type apiKeyClaims struct{ uid string }

func (c apiKeyClaims) GetUID() (string, error)     { return c.uid, nil }
func (c apiKeyClaims) GetSubject() (string, error) { return "", nil }
func (c apiKeyClaims) GetRoles() ([]string, error) { return nil, nil }
