package fileserver_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/fileserver"
	"github.com/go-sphere/sphere/storage/local"
)

// ExampleNewCDNAdapter wires a FileServer over the local driver, mounts its
// upload and download routes, and performs a one-time upload followed by a
// download, all against an in-process test server.
func ExampleNewCDNAdapter() {
	dir, err := os.MkdirTemp("", "fileserver-example-*")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()
	store, err := local.NewClient(local.Config{RootDir: dir})
	if err != nil {
		fmt.Println(err)
		return
	}

	engine := stdx.New()
	handler, ok := engine.(http.Handler)
	if !ok {
		fmt.Println("engine is not an http.Handler")
		return
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	fs, err := fileserver.NewCDNAdapter(fileserver.Config{
		PutBase:      server.URL + "/upload",
		GetBase:      server.URL + "/files",
		UploadNaming: storage.UploadNamingStrategyOriginal,
	}, memory.NewByteCache(), store, fileserver.WithOwnedCache())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = fs.Close() }()
	fs.RegisterFileUploader(engine.Group("/upload"))
	fs.RegisterFileDownloader(engine.Group("/files"))

	auth, err := fs.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		FileName: "hello.txt",
		Dir:      "docs",
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(auth.Authorization.Type, auth.Authorization.Method, auth.File.Key)

	put := func() int {
		req, err := http.NewRequest(auth.Authorization.Method, auth.Authorization.Value, bytes.NewReader([]byte("hello")))
		if err != nil {
			return 0
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			return 0
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	fmt.Println("first PUT:", put())
	fmt.Println("second PUT:", put()) // the token is single-use

	resp, err := server.Client().Get(auth.File.URL)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(resp.StatusCode, resp.Header.Get("Content-Disposition"), string(body))
	// Output:
	// url PUT docs/hello.txt
	// first PUT: 200
	// second PUT: 400
	// 200 attachment; filename=hello.txt hello
}
