package selector_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/server/middleware/selector"
)

// This example applies a guard only to the private operation of a generated
// route table.
func ExampleNewSelectorMiddleware() {
	// Generated code provides tables shaped like [operation, method, path].
	routes := [...][3]string{
		{"/user.v1.UserService/GetProfile", http.MethodGet, "/v1/profile"},
		{"/user.v1.UserService/Login", http.MethodPost, "/v1/login"},
	}
	requireToken := func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			if ctx.Header("Authorization") == "" {
				return httpx.NewUnauthorizedError("login required")
			}
			return next(ctx)
		}
	}

	engine := stdx.New(stdx.WithErrorHandler(httpz.AbortWithJsonError))
	api := engine.Group("/api")
	private := selector.MatchFunc(httpz.MatchOperation(api.BasePath(), routes[:],
		"/user.v1.UserService/GetProfile"))
	api.Use(selector.NewSelectorMiddleware(private, requireToken)...)
	api.GET("/v1/profile", httpz.WithText(func(httpx.Context) (string, error) { return "profile", nil }))
	api.POST("/v1/login", httpz.WithText(func(httpx.Context) (string, error) { return "token", nil }))

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil),
		httptest.NewRequest(http.MethodPost, "/api/v1/login", nil),
	} {
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
		fmt.Println(req.Method, req.URL.Path, resp.StatusCode, string(body))
	}
	// Output:
	// GET /api/v1/profile 401 {"success":false,"code":0,"message":"login required"}
	// POST /api/v1/login 200 token
}

func ExampleNewLogicalAndMatcher() {
	isAdmin := selector.NewContextMatcher("role", "admin")
	isBeta := selector.NewContextMatcher("beta", true)

	both := selector.NewLogicalAndMatcher(isAdmin, isBeta)
	either := selector.NewLogicalOrMatcher(isAdmin, isBeta)

	engine := stdx.New()
	engine.Group("").GET("/", func(ctx httpx.Context) error {
		ctx.Set("role", "admin")
		ctx.Set("beta", false)
		return ctx.Text(http.StatusOK, fmt.Sprintf("and=%t or=%t", both.Match(ctx), either.Match(ctx)))
	})

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	resp, err := tr.Do(httptest.NewRequest(http.MethodGet, "/", nil))
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
	fmt.Println(string(body))
	// Output:
	// and=false or=true
}
