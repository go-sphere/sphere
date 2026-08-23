package test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
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

// TestStorageDeleteIsIdempotent locks in the FileDeleter contract: removing a
// key that is not there succeeds instead of reporting ErrorNotFound, matching
// the object-store drivers whose native delete is already idempotent.
func TestStorageDeleteIsIdempotent(t *testing.T) {
	for _, factory := range storageFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			store := factory.new(t)

			if err := store.DeleteFile(ctx, "never/written.txt"); err != nil {
				t.Fatalf("DeleteFile(missing) error = %v, want nil", err)
			}

			if _, err := store.UploadFile(ctx, bytes.NewBufferString("body"), "gone/soon.txt"); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if err := store.DeleteFile(ctx, "gone/soon.txt"); err != nil {
				t.Fatalf("DeleteFile: %v", err)
			}
			// The second delete of the same key must behave like the first.
			if err := store.DeleteFile(ctx, "gone/soon.txt"); err != nil {
				t.Fatalf("DeleteFile(twice) error = %v, want nil", err)
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

// TestStorageKeyNormalization pins that every driver agrees on which keys are
// valid and which keys denote the same object.
//
// They used to disagree in ways that only showed up after switching backends:
// the local driver folded "a/" and "d//x" through filepath.Clean but returned
// the caller's raw key from UploadFile, so a key persisted in a database
// resolved on local — normalized again on the way back in — and 404'd on a
// backend that keeps keys verbatim. An empty key was rejected by local while
// kvcache happily stored an object reachable only by the empty string.
func TestStorageKeyNormalization(t *testing.T) {
	for _, factory := range storageFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			store := factory.new(t)

			t.Run("upload returns the key it stored under", func(t *testing.T) {
				for _, tc := range []struct{ given, want string }{
					{given: "plain.txt", want: "plain.txt"},
					{given: "/leading.txt", want: "leading.txt"},
					{given: "dir//double.txt", want: "dir/double.txt"},
					{given: "dir/./dot.txt", want: "dir/dot.txt"},
				} {
					got, err := store.UploadFile(ctx, strings.NewReader("body"), tc.given)
					if err != nil {
						t.Fatalf("UploadFile(%q): %v", tc.given, err)
					}
					if got != tc.want {
						t.Errorf("UploadFile(%q) returned key %q, want %q", tc.given, got, tc.want)
					}
					// The returned key must address the object it just wrote.
					if _, err := store.DownloadFile(ctx, got); err != nil {
						t.Errorf("the key returned for %q does not resolve: %v", tc.given, err)
					}
				}
			})

			t.Run("rejects keys that address nothing", func(t *testing.T) {
				for _, key := range []string{"", "/", ".", "..", "a/../../escape", "dir/.."} {
					if _, err := store.UploadFile(ctx, strings.NewReader("body"), key); err == nil {
						t.Errorf("UploadFile(%q) succeeded, want an error", key)
					}
				}
			})

			t.Run("equivalent spellings address one object", func(t *testing.T) {
				if _, err := store.UploadFile(ctx, strings.NewReader("first"), "shared/obj.txt"); err != nil {
					t.Fatalf("seed: %v", err)
				}
				for _, spelling := range []string{"/shared/obj.txt", "shared//obj.txt", "shared/./obj.txt"} {
					exists, err := store.IsFileExists(ctx, spelling)
					if err != nil {
						t.Fatalf("IsFileExists(%q): %v", spelling, err)
					}
					if !exists {
						t.Errorf("%q should address the same object as %q", spelling, "shared/obj.txt")
					}
				}
			})
		})
	}
}

func TestStorageMoveToSameKey(t *testing.T) {
	for _, factory := range storageFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ctx := context.Background()
			store := factory.new(t)

			// Missing file move to itself should return ErrNotFound
			err := store.MoveFile(ctx, "nonexistent.txt", "nonexistent.txt", true)
			if !errors.Is(err, storageerr.ErrNotFound) {
				t.Fatalf("MoveFile(missing to same) error = %v, want %v", err, storageerr.ErrNotFound)
			}

			// Existing file move to itself with overwrite=true should preserve the file
			const key = "self/move.txt"
			if _, err := store.UploadFile(ctx, bytes.NewBufferString("preserve-content"), key); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if err := store.MoveFile(ctx, key, key, true); err != nil {
				t.Fatalf("MoveFile(same key, overwrite=true): %v", err)
			}
			if got := readStorageFile(t, ctx, store, key); got != "preserve-content" {
				t.Fatalf("file content changed after moving to same key: got %q, want %q", got, "preserve-content")
			}
		})
	}
}
