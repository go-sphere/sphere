package docs_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/service/docs"
	"github.com/swaggo/swag"
)

// This example mounts the docs server on an httpx engine and calls an API
// through its reverse proxy.
func ExampleNewWebServerWithEngine() {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "api:"+r.URL.Path)
	}))
	defer api.Close()

	// Normally the swag-generated docs package provides this spec.
	spec := &swag.Spec{InfoInstanceName: "UserAPI", Title: "User API", Version: "v1"}

	engine := stdx.New()
	web, err := docs.NewWebServerWithEngine(docs.Config{
		Targets: []docs.Target{{Address: api.URL, Spec: spec}},
	}, engine)
	if err != nil {
		fmt.Println("docs:", err)
		return
	}
	fmt.Println(web.Identifier())

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	for _, path := range []string{"/", "/userapi/api/users/1"} {
		resp, err := tr.Do(httptest.NewRequest(http.MethodGet, path, nil))
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
		if resp.Header.Get("Content-Type") == "text/html" {
			fmt.Println(path, resp.StatusCode, "index page")
			continue
		}
		fmt.Println(path, resp.StatusCode, string(body))
	}
	// Output:
	// docs
	// / 200 index page
	// /userapi/api/users/1 200 api:/users/1
}
