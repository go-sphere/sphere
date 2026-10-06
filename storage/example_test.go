package storage_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/local"
	"github.com/go-sphere/sphere/storage/storageerr"
)

// newLocalStore returns a local driver rooted in a fresh temporary directory
// and a cleanup function that removes it.
func newLocalStore() (*local.Client, func(), error) {
	dir, err := os.MkdirTemp("", "storage-example-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	client, err := local.NewClient(local.Config{RootDir: dir})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return client, cleanup, nil
}

// Example shows the basic upload, download, and delete cycle against the
// Storage interface, using the local driver.
func Example() {
	client, cleanup, err := newLocalStore()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cleanup()

	ctx := context.Background()
	var store storage.Storage = client

	// The returned key is normalized; persist it rather than the argument.
	key, err := store.UploadFile(ctx, strings.NewReader("hello"), "/docs//hello.txt")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("key:", key)

	res, err := store.DownloadFile(ctx, key)
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
	fmt.Printf("body: %s (%d bytes, %s)\n", body, res.Size, res.MIME)

	if err := store.DeleteFile(ctx, key); err != nil {
		fmt.Println(err)
		return
	}
	// Deleting again succeeds: DeleteFile is idempotent.
	fmt.Println("delete again:", store.DeleteFile(ctx, key))
	// Output:
	// key: docs/hello.txt
	// body: hello (5 bytes, text/plain; charset=utf-8)
	// delete again: <nil>
}

// Example_missingKey shows how a missing object differs from an operational
// failure.
func Example_missingKey() {
	client, cleanup, err := newLocalStore()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cleanup()
	ctx := context.Background()

	exists, err := client.IsFileExists(ctx, "missing.txt")
	fmt.Println("exists:", exists, err)

	_, err = client.DownloadFile(ctx, "missing.txt")
	switch {
	case errors.Is(err, storageerr.ErrNotFound):
		fmt.Println("download: not found")
	case err != nil:
		fmt.Println("download failed:", err)
	}

	_, err = client.DownloadFile(ctx, "../etc/passwd")
	fmt.Println("traversal rejected:", errors.Is(err, storageerr.ErrFileNameInvalid))
	// Output:
	// exists: false <nil>
	// download: not found
	// traversal rejected: true
}

func ExampleNormalizeKey() {
	for _, key := range []string{"/a//b/./c.txt", "dir/", "a/../b", "/"} {
		normalized, err := storage.NormalizeKey(key)
		fmt.Printf("%q -> %q %v\n", key, normalized, err)
	}
	// Output:
	// "/a//b/./c.txt" -> "a/b/c.txt" <nil>
	// "dir/" -> "dir" <nil>
	// "a/../b" -> "" file name invalid
	// "/" -> "" file name invalid
}

func ExampleJoinUploadKey() {
	name, err := storage.BuildUploadFileName("photos/cat.png", storage.UploadNamingStrategyOriginal)
	if err != nil {
		fmt.Println(err)
		return
	}
	key, err := storage.JoinUploadKey("/uploads/", "avatars", name)
	fmt.Println(key, err)

	_, err = storage.JoinUploadKey("uploads", "../secrets", name)
	fmt.Println(err)
	// Output:
	// uploads/avatars/cat.png <nil>
	// biz_dir must not contain parent path
}

func ExampleResolveUploadTTL() {
	const hour = time.Hour
	// A request may shorten the configured lifetime but never extend it.
	fmt.Println(storage.ResolveUploadTTL(0, hour, 2*hour))
	fmt.Println(storage.ResolveUploadTTL(hour/2, hour, 2*hour))
	fmt.Println(storage.ResolveUploadTTL(3*hour, hour, 2*hour))
	fmt.Println(storage.ResolveUploadTTL(0, 0, 2*hour))
	// Output:
	// 1h0m0s
	// 30m0s
	// 1h0m0s
	// 2h0m0s
}

// ExampleFileLister pages through keys with the optional FileLister
// capability, probing for it with a type assertion.
func ExampleFileLister() {
	client, cleanup, err := newLocalStore()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cleanup()
	ctx := context.Background()

	for _, key := range []string{"img/a.png", "img/b.png", "img/c.png", "doc/d.txt"} {
		if _, err := client.UploadFile(ctx, strings.NewReader("x"), key); err != nil {
			fmt.Println(err)
			return
		}
	}

	var store storage.Storage = client
	lister, ok := store.(storage.FileLister)
	if !ok {
		fmt.Println("listing not supported")
		return
	}
	cursor := ""
	for {
		keys, next, err := lister.ListFiles(ctx, "img/", cursor, 2)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(keys)
		// Stop on an empty cursor, not on a short or empty page.
		if next == "" {
			break
		}
		cursor = next
	}
	// Output:
	// [img/a.png img/b.png]
	// [img/c.png]
}
