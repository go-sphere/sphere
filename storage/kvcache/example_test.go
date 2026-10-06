package kvcache_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage/kvcache"
	"github.com/go-sphere/sphere/storage/storageerr"
)

func ExampleNewClient() {
	// The cache is owned by the caller; kvcache never closes it.
	byteCache := memory.NewByteCache()
	defer func() { _ = byteCache.Close() }()

	client, err := kvcache.NewClient(kvcache.Config{}, byteCache)
	if err != nil {
		fmt.Println(err)
		return
	}
	ctx := context.Background()

	key, err := client.UploadFile(ctx, strings.NewReader(`{"ok":true}`), "/tmp/data.json")
	if err != nil {
		fmt.Println(err)
		return
	}
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
	fmt.Println(key, res.MIME, string(body))

	if err := client.DeleteFile(ctx, key); err != nil {
		fmt.Println(err)
		return
	}
	_, err = client.DownloadFile(ctx, key)
	fmt.Println("after delete:", errors.Is(err, storageerr.ErrNotFound))
	// Output:
	// tmp/data.json application/json {"ok":true}
	// after delete: true
}
