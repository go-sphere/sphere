package kvcache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage/storageerr"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	client, err := NewClient(&Config{
		PublicBase: "http://localhost",
	}, memory.NewByteCache())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestClientDownloadFileNotFound(t *testing.T) {
	client := newTestClient(t)
	read, _, _, err := client.DownloadFile(context.Background(), "missing.txt")
	if !errors.Is(err, storageerr.ErrorNotFound) {
		t.Fatalf("DownloadFile() error = %v, want %v", err, storageerr.ErrorNotFound)
	}
	if read != nil {
		t.Fatalf("DownloadFile() reader = %v, want nil", read)
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
		read, _, _, downErr := client.DownloadFile(ctx, "new-destination.txt")
		if downErr != nil {
			t.Fatalf("DownloadFile() error = %v", downErr)
		}
		defer func() {
			_ = read.Close()
		}()
		all, readErr := io.ReadAll(read)
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
		if !errors.Is(copyErr, storageerr.ErrorDistExisted) {
			t.Fatalf("CopyFile() error = %v, want %v", copyErr, storageerr.ErrorDistExisted)
		}
	})
}
