package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/fileserver"
	"github.com/go-sphere/sphere/storage/kvcache"
	"github.com/go-sphere/sphere/storage/local"
	"github.com/go-sphere/sphere/storage/qiniu"
	"github.com/go-sphere/sphere/storage/s3"
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
// key that is not there succeeds instead of reporting ErrNotFound, matching
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
			if !errors.Is(err, storageerr.ErrNotFound) {
				t.Fatalf("CopyFile missing source error = %v, want %v", err, storageerr.ErrNotFound)
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
			if !errors.Is(err, storageerr.ErrDestExists) {
				t.Fatalf("CopyFile no-overwrite error = %v, want %v", err, storageerr.ErrDestExists)
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
					download, err := store.DownloadFile(ctx, got)
					if err != nil {
						t.Errorf("the key returned for %q does not resolve: %v", tc.given, err)
						continue
					}
					if err := download.Reader.Close(); err != nil {
						t.Errorf("close DownloadFile(%q): %v", got, err)
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

// failingReader simulates a stream interruption by failing with an error after reading N bytes.
type failingReader struct {
	data      []byte
	offset    int
	failAfter int
	err       error
}

func (r *failingReader) Read(p []byte) (n int, err error) {
	if r.offset >= r.failAfter {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.ErrUnexpectedEOF
	}
	remaining := r.failAfter - r.offset
	toRead := min(min(len(p), remaining), len(r.data)-r.offset)
	copy(p, r.data[r.offset:r.offset+toRead])
	r.offset += toRead
	if r.offset >= r.failAfter {
		if r.err != nil {
			return toRead, r.err
		}
		return toRead, io.ErrUnexpectedEOF
	}
	return toRead, nil
}

// TestStorageConcurrentUploadDownload verifies that independent writes retain
// their bytes and shared-key overwrites are observed only as complete payloads.
func TestStorageConcurrentUploadDownload(t *testing.T) {
	for _, factory := range storageFactories() {
		t.Run(factory.name, func(t *testing.T) {
			store := factory.new(t)
			ctx := t.Context()

			const concurrency = 8
			const opsPerWorker = 8
			const sharedKey = "concurrent/shared.dat"
			validSharedPayloads := map[string]struct{}{"seed": {}}
			for workerID := range concurrency {
				for op := range opsPerWorker {
					payload := fmt.Sprintf("worker-%d-op-%d-payload", workerID, op)
					validSharedPayloads[payload] = struct{}{}
				}
			}
			if _, err := store.UploadFile(ctx, strings.NewReader("seed"), sharedKey); err != nil {
				t.Fatalf("seed shared key: %v", err)
			}

			var wg sync.WaitGroup
			errCh := make(chan error, concurrency*opsPerWorker)

			for workerID := range concurrency {
				wg.Go(func() {
					for op := range opsPerWorker {
						key := fmt.Sprintf("concurrent/worker_%d/file_%d.dat", workerID, op)
						payload := []byte(fmt.Sprintf("private-worker-%d-op-%d", workerID, op))

						uploadedKey, err := store.UploadFile(ctx, bytes.NewReader(payload), key)
						if err != nil {
							errCh <- fmt.Errorf("worker %d op %d upload error: %w", workerID, op, err)
							return
						}
						if uploadedKey != key {
							errCh <- fmt.Errorf("worker %d op %d upload key mismatch: got %q, want %q", workerID, op, uploadedKey, key)
							return
						}

						res, err := store.DownloadFile(ctx, key)
						if err != nil {
							errCh <- fmt.Errorf("worker %d op %d download error: %w", workerID, op, err)
							return
						}
						data, err := io.ReadAll(res.Reader)
						closeErr := res.Reader.Close()
						if err != nil {
							errCh <- fmt.Errorf("worker %d op %d read download error: %w", workerID, op, err)
							return
						}
						if closeErr != nil {
							errCh <- fmt.Errorf("worker %d op %d close download: %w", workerID, op, closeErr)
							return
						}
						if !bytes.Equal(data, payload) {
							errCh <- fmt.Errorf("worker %d op %d private payload = %q, want %q", workerID, op, data, payload)
							return
						}

						sharedPayload := []byte(fmt.Sprintf("worker-%d-op-%d-payload", workerID, op))
						_, err = store.UploadFile(ctx, bytes.NewReader(sharedPayload), sharedKey)
						if err != nil {
							errCh <- fmt.Errorf("worker %d shared upload error: %w", workerID, err)
							return
						}

						sRes, err := store.DownloadFile(ctx, sharedKey)
						if err != nil {
							errCh <- fmt.Errorf("worker %d shared download error: %w", workerID, err)
							return
						}
						sData, readErr := io.ReadAll(sRes.Reader)
						closeErr = sRes.Reader.Close()
						if readErr != nil {
							errCh <- fmt.Errorf("worker %d read shared error: %w", workerID, readErr)
							return
						}
						if closeErr != nil {
							errCh <- fmt.Errorf("worker %d close shared download: %w", workerID, closeErr)
							return
						}
						if _, ok := validSharedPayloads[string(sData)]; !ok {
							errCh <- fmt.Errorf("worker %d observed partial shared payload %q", workerID, sData)
							return
						}
					}
				})
			}

			wg.Wait()
			close(errCh)

			for err := range errCh {
				t.Errorf("concurrent test failure: %v", err)
			}
		})
	}
}

// TestStorageStreamInterruptionAndAtomicCleanup verifies that when a stream fails mid-copy,
// the operation fails cleanly, does not leave partial data at destination, and does not leak tmp files.
func TestStorageStreamInterruptionAndAtomicCleanup(t *testing.T) {
	tempDir := t.TempDir()
	client, err := local.NewClient(local.Config{RootDir: tempDir})
	if err != nil {
		t.Fatalf("new local client: %v", err)
	}
	ctx := context.Background()

	t.Run("failing reader on new key leaves no destination and no tmp files", func(t *testing.T) {
		key := "interrupted/new_file.txt"
		badReader := &failingReader{
			data:      bytes.Repeat([]byte("A"), 1024*64),
			failAfter: 1024 * 16,
			err:       errors.New("simulated network drop"),
		}

		_, err := client.UploadFile(ctx, badReader, key)
		if err == nil {
			t.Fatal("expected UploadFile to fail with badReader, but got nil")
		}

		exists, err := client.IsFileExists(ctx, key)
		if err != nil {
			t.Fatalf("IsFileExists error: %v", err)
		}
		if exists {
			t.Fatal("destination file exists despite upload interruption!")
		}

		// Verify no temporary files left in RootDir
		entries, err := os.ReadDir(filepath.Join(tempDir, "interrupted"))
		if err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), ".sphere-tmp-") {
					t.Fatalf("orphaned tmp file found: %s", e.Name())
				}
			}
		}
	})

	t.Run("failing reader on overwrite preserves previous intact file", func(t *testing.T) {
		key := "interrupted/overwrite_file.txt"
		originalContent := "original indestructible content"
		if _, err := client.UploadFile(ctx, strings.NewReader(originalContent), key); err != nil {
			t.Fatalf("seed upload: %v", err)
		}

		badReader := &failingReader{
			data:      bytes.Repeat([]byte("B"), 1024*64),
			failAfter: 1024 * 32,
			err:       errors.New("simulated crash mid stream"),
		}

		_, err := client.UploadFile(ctx, badReader, key)
		if err == nil {
			t.Fatal("expected UploadFile to fail on badReader")
		}

		// Check original content is still 100% intact
		res, err := client.DownloadFile(ctx, key)
		if err != nil {
			t.Fatalf("DownloadFile after failed overwrite: %v", err)
		}
		defer func() { _ = res.Reader.Close() }()
		data, err := io.ReadAll(res.Reader)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(data) != originalContent {
			t.Fatalf("content was altered after failed overwrite: got %q, want %q", string(data), originalContent)
		}
	})

	t.Run("cancelled context before upload fails immediately", func(t *testing.T) {
		key := "interrupted/cancelled.txt"
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		_, err := client.UploadFile(cancelCtx, strings.NewReader("some content"), key)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}

		exists, err := client.IsFileExists(context.Background(), key)
		if err != nil {
			t.Fatalf("IsFileExists error: %v", err)
		}
		if exists {
			t.Fatal("destination exists after cancelled upload!")
		}
	})
}

// TestStoragePathTraversalRejected verifies every local operation rejects an
// invalid key and cannot modify a sentinel beside RootDir.
func TestStoragePathTraversalRejected(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	client, err := local.NewClient(local.Config{RootDir: root})
	if err != nil {
		t.Fatalf("new local client: %v", err)
	}
	ctx := t.Context()
	const sentinelBody = "outside storage root"
	sentinelPath := filepath.Join(parent, "sentinel.txt")
	if err := os.WriteFile(sentinelPath, []byte(sentinelBody), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	if _, err := client.UploadFile(ctx, strings.NewReader("safe source"), "safe/source.txt"); err != nil {
		t.Fatalf("seed safe source: %v", err)
	}

	traversalPayloads := []string{
		"../sentinel.txt",
		"../../sentinel.txt",
		"..",
		"/..",
		"dir/..",
		"dir/../../escape",
		"dir//..//secret.txt",
		"",
		"/",
		".",
	}

	for _, payload := range traversalPayloads {
		t.Run(fmt.Sprintf("key=%q", payload), func(t *testing.T) {
			assertInvalid := func(operation string, err error) {
				t.Helper()
				if !errors.Is(err, storageerr.ErrFileNameInvalid) {
					t.Errorf("%s error = %v, want %v", operation, err, storageerr.ErrFileNameInvalid)
				}
			}

			_, err := storage.NormalizeKey(payload)
			assertInvalid("NormalizeKey", err)
			_, err = client.UploadFile(ctx, strings.NewReader("malicious"), payload)
			assertInvalid("UploadFile", err)
			_, err = client.DownloadFile(ctx, payload)
			assertInvalid("DownloadFile", err)
			_, err = client.IsFileExists(ctx, payload)
			assertInvalid("IsFileExists", err)
			_, err = client.StatFile(ctx, payload)
			assertInvalid("StatFile", err)
			assertInvalid("DeleteFile", client.DeleteFile(ctx, payload))
			assertInvalid("CopyFile source", client.CopyFile(ctx, payload, "safe/copy.txt", true))
			assertInvalid("CopyFile destination", client.CopyFile(ctx, "safe/source.txt", payload, true))
			assertInvalid("MoveFile source", client.MoveFile(ctx, payload, "safe/move.txt", true))
			assertInvalid("MoveFile destination", client.MoveFile(ctx, "safe/source.txt", payload, true))

			body, readErr := os.ReadFile(sentinelPath)
			if readErr != nil {
				t.Fatalf("read outside sentinel: %v", readErr)
			}
			if string(body) != sentinelBody {
				t.Fatalf("outside sentinel changed: got %q, want %q", body, sentinelBody)
			}
		})
	}
}

func TestLocalDownloadReaderCanBeClosed(t *testing.T) {
	client, err := local.NewClient(local.Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("new local client: %v", err)
	}
	if _, err := client.UploadFile(t.Context(), strings.NewReader("body"), "file.txt"); err != nil {
		t.Fatalf("seed upload: %v", err)
	}

	for range 8 {
		result, err := client.DownloadFile(t.Context(), "file.txt")
		if err != nil {
			t.Fatalf("DownloadFile: %v", err)
		}
		if err := result.Reader.Close(); err != nil {
			t.Fatalf("close download reader: %v", err)
		}
		if _, err := result.Reader.Read(make([]byte, 1)); err == nil {
			t.Fatal("read after Close succeeded")
		}
	}
}

var _ storage.CDNStorage = (*s3.Client)(nil)

var _ storage.CDNStorage = (*qiniu.Client)(nil)

var _ storage.CDNStorage = (*fileserver.FileServer)(nil)

var _ storage.Storage = (*local.Client)(nil)

var _ storage.Storage = (*kvcache.Client)(nil)

// The optional FileStater / FileLister capabilities are implemented by the
// backends that can serve them cheaply (local, s3, qiniu) and intentionally
// omitted from kvcache and the fileserver adapter.
var (
	_ storage.FileStater = (*local.Client)(nil)
	_ storage.FileStater = (*s3.Client)(nil)
	_ storage.FileStater = (*qiniu.Client)(nil)
	_ storage.FileLister = (*local.Client)(nil)
	_ storage.FileLister = (*s3.Client)(nil)
	_ storage.FileLister = (*qiniu.Client)(nil)
)
