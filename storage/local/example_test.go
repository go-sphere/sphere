package local_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-sphere/sphere/storage/local"
	"github.com/go-sphere/sphere/storage/storageerr"
)

func ExampleNewClient() {
	dir, err := os.MkdirTemp("", "local-example-*")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	client, err := local.NewClient(local.Config{RootDir: dir})
	if err != nil {
		fmt.Println(err)
		return
	}
	ctx := context.Background()

	key, err := client.UploadFile(ctx, strings.NewReader("hello"), "docs/hello.txt")
	if err != nil {
		fmt.Println(err)
		return
	}
	info, err := client.StatFile(ctx, key)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(key, info.Size, info.MIME)

	res, err := client.DownloadFile(ctx, key)
	if err != nil {
		fmt.Println(err)
		return
	}
	body, err := io.ReadAll(res.Reader)
	_ = res.Reader.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(body))
	// Output:
	// docs/hello.txt 5 text/plain; charset=utf-8
	// hello
}

func ExampleClient_MoveFile() {
	dir, err := os.MkdirTemp("", "local-example-*")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	client, err := local.NewClient(local.Config{RootDir: dir})
	if err != nil {
		fmt.Println(err)
		return
	}
	ctx := context.Background()
	for _, key := range []string{"a.txt", "b.txt"} {
		if _, err := client.UploadFile(ctx, strings.NewReader(key), key); err != nil {
			fmt.Println(err)
			return
		}
	}

	err = client.MoveFile(ctx, "a.txt", "b.txt", false)
	fmt.Println("no overwrite:", errors.Is(err, storageerr.ErrDestExists))

	err = client.MoveFile(ctx, "a.txt", "b.txt", true)
	fmt.Println("overwrite:", err)

	exists, err := client.IsFileExists(ctx, "a.txt")
	fmt.Println("source exists:", exists, err)
	// Output:
	// no overwrite: true
	// overwrite: <nil>
	// source exists: false <nil>
}
