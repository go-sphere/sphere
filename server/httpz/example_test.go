package httpz_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/httpz"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// do dispatches req in-process and prints the status and body.
func do(engine httpx.Engine, req *http.Request) {
	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	resp, err := tr.Do(req)
	if err != nil {
		fmt.Println("request failed:", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}
	fmt.Println(resp.StatusCode, string(bytes.TrimSpace(body)))
}

func Example() {
	users := map[string]User{"1": {ID: "1", Name: "Ada"}}

	engine := stdx.New(stdx.WithErrorHandler(httpz.AbortWithJsonError))
	api := engine.Group("/api")
	api.GET("/users/:id", httpz.WithJson(func(ctx httpx.Context) (User, error) {
		user, ok := users[ctx.Param("id")]
		if !ok {
			return User{}, httpx.NewNotFoundError("user not found")
		}
		return user, nil
	}))

	do(engine, httptest.NewRequest(http.MethodGet, "/api/users/1", nil))
	do(engine, httptest.NewRequest(http.MethodGet, "/api/users/2", nil))
	// Output:
	// 200 {"success":true,"data":{"id":"1","name":"Ada"}}
	// 404 {"success":false,"code":0,"message":"user not found"}
}

func ExampleWithJson_createdStatus() {
	engine := stdx.New()
	engine.Group("").POST("/users", httpz.WithJson(func(ctx httpx.Context) (User, error) {
		ctx.Status(http.StatusCreated)
		return User{ID: "2", Name: "Grace"}, nil
	}))

	do(engine, httptest.NewRequest(http.MethodPost, "/users", nil))
	// Output:
	// 201 {"success":true,"data":{"id":"2","name":"Grace"}}
}

func ExampleSetDefaultErrorParser() {
	errQuota := errors.New("quota exceeded")
	httpz.SetDefaultErrorParser(func(err error) (int32, int32, string) {
		if errors.Is(err, errQuota) {
			return 1001, http.StatusTooManyRequests, "try again later"
		}
		return httpz.ParseError(err)
	})
	defer httpz.SetDefaultErrorParser(httpz.ParseError)

	engine := stdx.New()
	engine.Group("").GET("/quota", httpz.WithJson(func(httpx.Context) (struct{}, error) {
		return struct{}{}, errQuota
	}))

	do(engine, httptest.NewRequest(http.MethodGet, "/quota", nil))
	// Output:
	// 429 {"success":false,"code":1001,"message":"try again later"}
}

func ExampleWithFormFileBytes() {
	engine := stdx.New()
	engine.Group("").POST("/upload", httpz.WithFormFileBytes(
		func(ctx httpx.Context, file []byte, filename string) (string, error) {
			return fmt.Sprintf("%s: %d bytes", filename, len(file)), nil
		},
		httpz.WithFormAllowExtensions(".txt"),
		httpz.WithFormMaxSize(1<<20),
	))

	upload := func(filename, content string) *http.Request {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, err := form.CreateFormFile("file", filename)
		if err != nil {
			panic(err)
		}
		if _, err := io.WriteString(part, content); err != nil {
			panic(err)
		}
		if err := form.Close(); err != nil {
			panic(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/upload", &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		return req
	}

	do(engine, upload("notes.txt", "hello"))
	do(engine, upload("tool.exe", "MZ"))
	// Output:
	// 200 {"success":true,"data":"notes.txt: 5 bytes"}
	// 400 {"success":false,"code":0,"message":"File extension not allowed: .exe"}
}

func ExampleWithSSE() {
	engine := stdx.New()
	engine.Group("").GET("/count", httpz.WithSSE(func(ctx httpx.Context) (httpz.SSEStream[int], error) {
		// Phase one: bind and validate on the handler goroutine.
		limit := 3
		// Phase two: produce messages; do not touch ctx here.
		return func(send func(int) error) error {
			for i := 1; i <= limit; i++ {
				if err := send(i); err != nil {
					return err // client gone
				}
			}
			return nil
		}, nil
	}))

	do(engine, httptest.NewRequest(http.MethodGet, "/count", nil))
	// Output:
	// 200 data: 1
	//
	// data: 2
	//
	// data: 3
	//
	// event: done
	// data: {}
}

func ExampleMatchOperation() {
	// Route tables like this one are generated as [operation, method, path].
	routes := [][3]string{
		{"/user.v1.UserService/GetProfile", http.MethodGet, "/v1/profile"},
		{"/user.v1.UserService/Login", http.MethodPost, "/v1/login"},
	}
	private := httpz.MatchOperation("/api", routes, "/user.v1.UserService/GetProfile")

	engine := stdx.New(stdx.WithErrorHandler(httpz.AbortWithJsonError))
	api := engine.Group("/api", func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			if private(ctx) && ctx.Header("Authorization") == "" {
				return httpx.NewUnauthorizedError("login required")
			}
			return next(ctx)
		}
	})
	api.GET("/v1/profile", httpz.WithText(func(httpx.Context) (string, error) { return "profile", nil }))
	api.POST("/v1/login", httpz.WithText(func(httpx.Context) (string, error) { return "token", nil }))

	do(engine, httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil))
	do(engine, httptest.NewRequest(http.MethodPost, "/api/v1/login", nil))
	// Output:
	// 401 {"success":false,"code":0,"message":"login required"}
	// 200 token
}

func ExampleMountStdAll() {
	engine := stdx.New()
	health := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	if err := httpz.MountStdAll(engine.Group(""), "/healthz", health, http.MethodGet); err != nil {
		fmt.Println("mount failed:", err)
		return
	}

	do(engine, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	// Output:
	// 200 ok
}
