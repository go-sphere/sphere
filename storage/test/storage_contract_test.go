package test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/kvcache"
	"github.com/go-sphere/sphere/storage/local"
	"github.com/go-sphere/sphere/storage/storageerr"
)

type storageFactory struct {
	name string
	new  func(*testing.T) storage.Storage
}

func storageFactories() []storageFactory {
	return []storageFactory{
		{
			name: "local",
			new: func(t *testing.T) storage.Storage {
				t.Helper()
				client, err := local.NewClient(local.Config{RootDir: t.TempDir()})
				if err != nil {
					t.Fatalf("new local storage: %v", err)
				}
				return client
			},
		},
		{
			name: "kvcache",
			new: func(t *testing.T) storage.Storage {
				t.Helper()
				cache := memory.NewByteCache()
				t.Cleanup(func() { _ = cache.Close() })
				client, err := kvcache.NewClient(kvcache.Config{}, cache)
				if err != nil {
					t.Fatalf("new kvcache storage: %v", err)
				}
				return client
			},
		},
	}
}

func TestStorageCoreContract(t *testing.T) {
	for _, factory := range storageFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			store := factory.new(t)

			key, err := store.UploadFile(ctx, bytes.NewBufferString("source"), "docs/source.txt")
			if err != nil {
				t.Fatalf("UploadFile: %v", err)
			}
			if key != "docs/source.txt" {
				t.Fatalf("UploadFile key = %q, want %q", key, "docs/source.txt")
			}

			exists, err := store.IsFileExists(ctx, key)
			if err != nil {
				t.Fatalf("IsFileExists: %v", err)
			}
			if !exists {
				t.Fatal("uploaded key does not exist")
			}
			if got := readStorageFile(t, ctx, store, key); got != "source" {
				t.Fatalf("download = %q, want %q", got, "source")
			}

			if err := store.CopyFile(ctx, key, "docs/copy.txt", false); err != nil {
				t.Fatalf("CopyFile: %v", err)
			}
			if got := readStorageFile(t, ctx, store, "docs/copy.txt"); got != "source" {
				t.Fatalf("copy = %q, want %q", got, "source")
			}

			if err := store.MoveFile(ctx, "docs/copy.txt", "docs/moved.txt", false); err != nil {
				t.Fatalf("MoveFile: %v", err)
			}
			if exists, err := store.IsFileExists(ctx, "docs/copy.txt"); err != nil || exists {
				t.Fatalf("source after MoveFile: exists=%v err=%v", exists, err)
			}
			if got := readStorageFile(t, ctx, store, "docs/moved.txt"); got != "source" {
				t.Fatalf("moved = %q, want %q", got, "source")
			}

			if err := store.DeleteFile(ctx, "docs/moved.txt"); err != nil {
				t.Fatalf("DeleteFile: %v", err)
			}
			if exists, err := store.IsFileExists(ctx, "docs/moved.txt"); err != nil || exists {
				t.Fatalf("deleted key: exists=%v err=%v", exists, err)
			}
		})
	}
}

func TestStorageCopyFailurePreservesDestination(t *testing.T) {
	for _, factory := range storageFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			store := factory.new(t)
			if _, err := store.UploadFile(ctx, bytes.NewBufferString("current"), "release/live.txt"); err != nil {
				t.Fatalf("seed destination: %v", err)
			}

			err := store.CopyFile(ctx, "release/missing.txt", "release/live.txt", true)
			if !errors.Is(err, storageerr.ErrorNotFound) {
				t.Fatalf("CopyFile missing source error = %v, want %v", err, storageerr.ErrorNotFound)
			}
			if got := readStorageFile(t, ctx, store, "release/live.txt"); got != "current" {
				t.Fatalf("destination changed after failed copy: %q", got)
			}
		})
	}
}

func TestStorageNoOverwritePreservesDestination(t *testing.T) {
	for _, factory := range storageFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			store := factory.new(t)
			if _, err := store.UploadFile(ctx, bytes.NewBufferString("source"), "copy/source.txt"); err != nil {
				t.Fatalf("seed source: %v", err)
			}
			if _, err := store.UploadFile(ctx, bytes.NewBufferString("destination"), "copy/destination.txt"); err != nil {
				t.Fatalf("seed destination: %v", err)
			}

			err := store.CopyFile(ctx, "copy/source.txt", "copy/destination.txt", false)
			if !errors.Is(err, storageerr.ErrorDistExisted) {
				t.Fatalf("CopyFile no-overwrite error = %v, want %v", err, storageerr.ErrorDistExisted)
			}
			if got := readStorageFile(t, ctx, store, "copy/destination.txt"); got != "destination" {
				t.Fatalf("destination was overwritten: %q", got)
			}
		})
	}
}

func readStorageFile(t *testing.T, ctx context.Context, store storage.Storage, key string) string {
	t.Helper()
	result, err := store.DownloadFile(ctx, key)
	if err != nil {
		t.Fatalf("DownloadFile(%q): %v", key, err)
	}
	defer func() { _ = result.Reader.Close() }()
	data, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("ReadAll(%q): %v", key, err)
	}
	if result.Size != int64(len(data)) {
		t.Fatalf("DownloadFile(%q) size = %d, want %d", key, result.Size, len(data))
	}
	return string(data)
}
