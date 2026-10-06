package file_test

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/server/service/file"
	"github.com/go-sphere/sphere/storage"
)

func ExampleNewLocalFileService() {
	dir, err := os.MkdirTemp("", "file-example")
	if err != nil {
		fmt.Println("temp dir:", err)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	files, err := file.NewLocalFileService(file.LocalFileServiceConfig{
		RootDir:    dir,
		PublicBase: "http://localhost:9000/",
	})
	if err != nil {
		fmt.Println("file service:", err)
		return
	}
	// Web.Stop closes it when the adapter is served; here it is used alone.
	defer func() { _ = files.Close() }()

	auth, err := files.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		FileName: "avatar.png",
	})
	if err != nil {
		fmt.Println("upload auth:", err)
		return
	}
	fmt.Println(auth.Authorization.Method, strings.HasPrefix(auth.Authorization.Value, "http://localhost:9000/"))
	fmt.Println(auth.File.URL == files.GenerateURL(auth.File.Key))
	// Output:
	// PUT true
	// true
}

// This example wires the service into an application started by core/boot.
// It has no output check because boot.Run listens on a TCP port and waits for
// a shutdown signal.
func ExampleNewWebServer() {
	type Config struct {
		Addr  string
		Files file.LocalFileServiceConfig
	}
	conf := &Config{
		Addr: ":9000",
		Files: file.LocalFileServiceConfig{
			RootDir:    "./data/files",
			PublicBase: "http://localhost:9000/",
		},
	}
	err := boot.Run(conf, func(conf *Config) (*boot.Application, error) {
		files, err := file.NewLocalFileService(conf.Files)
		if err != nil {
			return nil, err
		}
		web := file.NewWebServer(stdx.New(stdx.WithAddr(conf.Addr)), files)
		return boot.NewApplication(web), nil
	})
	if err != nil {
		fmt.Println("run:", err)
	}
}
