package kvcache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage/storageerr"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	c := memory.NewByteCache()
	t.Cleanup(func() { _ = c.Close() })
	client, err := NewClient(Config{}, c)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestClientDownloadFileNotFound(t *testing.T) {
	client := newTestClient(t)
	result, err := client.DownloadFile(context.Background(), "missing.txt")
	if !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("DownloadFile() error = %v, want %v", err, storageerr.ErrNotFound)
	}
	if result.Reader != nil {
		t.Fatalf("DownloadFile() reader = %v, want nil", result.Reader)
	}
}

func TestClientCopyFileOverwriteBehavior(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	_, err := client.UploadFile(ctx, bytes.NewBufferString("source"), "source.txt")
	if err != nil {
		t.Fatalf("UploadFile(source) error = %v", err)
	}

	t.Run("destination missing and overwrite false should succeed", func(t *testing.T) {
		copyErr := client.CopyFile(ctx, "source.txt", "new-destination.txt", false)
		if copyErr != nil {
			t.Fatalf("CopyFile() error = %v", copyErr)
		}
		result, downErr := client.DownloadFile(ctx, "new-destination.txt")
		if downErr != nil {
			t.Fatalf("DownloadFile() error = %v", downErr)
		}
		defer func() {
			_ = result.Reader.Close()
		}()
		all, readErr := io.ReadAll(result.Reader)
		if readErr != nil {
			t.Fatalf("ReadAll() error = %v", readErr)
		}
		if string(all) != "source" {
			t.Fatalf("copied content = %q, want %q", string(all), "source")
		}
	})

	t.Run("destination exists and overwrite false should fail", func(t *testing.T) {
		_, upErr := client.UploadFile(ctx, bytes.NewBufferString("destination"), "existing.txt")
		if upErr != nil {
			t.Fatalf("UploadFile(destination) error = %v", upErr)
		}
		copyErr := client.CopyFile(ctx, "source.txt", "existing.txt", false)
		if !errors.Is(copyErr, storageerr.ErrDestExists) {
			t.Fatalf("CopyFile() error = %v, want %v", copyErr, storageerr.ErrDestExists)
		}
	})

	t.Run("same key copy", func(t *testing.T) {
		if err := client.CopyFile(ctx, "missing.txt", "missing.txt", true); !errors.Is(err, storageerr.ErrNotFound) {
			t.Fatalf("CopyFile(missing same) error = %v, want ErrNotFound", err)
		}
		if err := client.CopyFile(ctx, "source.txt", "source.txt", false); !errors.Is(err, storageerr.ErrDestExists) {
			t.Fatalf("CopyFile(existing same, overwrite=false) error = %v, want ErrDestExists", err)
		}
		if err := client.CopyFile(ctx, "source.txt", "source.txt", true); err != nil {
			t.Fatalf("CopyFile(existing same, overwrite=true) error = %v, want nil", err)
		}
	})

	t.Run("nil reader and canceled ctx", func(t *testing.T) {
		if _, err := client.UploadFile(ctx, nil, "nil.txt"); err == nil {
			t.Fatal("UploadFile(nil reader) succeeded, want error")
		}
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := client.UploadFile(canceledCtx, bytes.NewBufferString("x"), "canceled.txt"); !errors.Is(err, context.Canceled) {
			t.Fatalf("UploadFile(canceled) error = %v, want context.Canceled", err)
		}
		var nilCtx context.Context
		if _, err := client.UploadFile(nilCtx, bytes.NewBufferString("ok"), "nilctx.txt"); err != nil {
			t.Fatalf("UploadFile(nil ctx) error = %v", err)
		}
	})
}

func TestNewClientRequiresNonNilCache(t *testing.T) {
	if _, err := NewClient(Config{}, nil); err == nil {
		t.Fatal("NewClient(nil) succeeded, want error")
	}
	var typedNil *mcache.Map[string, []byte]
	if _, err := NewClient(Config{}, typedNil); err == nil {
		t.Fatal("NewClient(typed nil) succeeded, want error")
	}
}
