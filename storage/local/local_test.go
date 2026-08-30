package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sphere/sphere/storage/storageerr"
)

func TestClient_fixFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "data")

	client, err := NewClient(Config{
		RootDir: rootDir,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	t.Run("allow path within root", func(t *testing.T) {
		got, fixErr := client.fixFilePath(filepath.Join("images", "a.png"))
		if fixErr != nil {
			t.Fatalf("fixFilePath() error = %v", fixErr)
		}
		want := filepath.Join(rootDir, "images", "a.png")
		if filepath.Clean(got) != filepath.Clean(want) {
			t.Fatalf("fixFilePath() = %q, want %q", got, want)
		}
	})

	t.Run("reject sibling directory traversal", func(t *testing.T) {
		_, fixErr := client.fixFilePath(filepath.Join("..", "data2", "a.png"))
		if !errors.Is(fixErr, storageerr.ErrFileNameInvalid) {
			t.Fatalf("fixFilePath() error = %v, want %v", fixErr, storageerr.ErrFileNameInvalid)
		}
	})

	t.Run("reject empty and root keys", func(t *testing.T) {
		for _, key := range []string{"", ".", "/"} {
			_, fixErr := client.fixFilePath(key)
			if !errors.Is(fixErr, storageerr.ErrFileNameInvalid) {
				t.Fatalf("fixFilePath(%q) error = %v, want %v", key, fixErr, storageerr.ErrFileNameInvalid)
			}
		}
	})
}

// failingReader returns some bytes and then an error, to simulate a stream that
// fails partway through an upload.
type failingReader struct {
	data []byte
	err  error
}

func (r *failingReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return 0, r.err
}

func TestClient_UploadFileFailureLeavesNoResidual(t *testing.T) {
	client, err := NewClient(Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	const key = "images/broken.bin"
	wantErr := errors.New("boom")
	_, err = client.UploadFile(context.Background(), &failingReader{data: []byte("partial"), err: wantErr}, key)
	if !errors.Is(err, wantErr) {
		t.Fatalf("UploadFile() error = %v, want %v", err, wantErr)
	}

	filePath, fixErr := client.fixFilePath(key)
	if fixErr != nil {
		t.Fatalf("fixFilePath() error = %v", fixErr)
	}
	if _, statErr := os.Stat(filePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no residual file, stat err = %v", statErr)
	}
}

// TestClient_UploadFileFailurePreservesExisting covers the atomic-write
// guarantee: a failed overwrite must leave the previous contents readable
// rather than truncating or removing the key.
func TestClient_UploadFileFailurePreservesExisting(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	const key = "images/keep.bin"
	if _, e := client.UploadFile(ctx, strings.NewReader("original"), key); e != nil {
		t.Fatalf("seed upload: %v", e)
	}

	wantErr := errors.New("boom")
	if _, e := client.UploadFile(ctx, &failingReader{data: []byte("partial"), err: wantErr}, key); !errors.Is(e, wantErr) {
		t.Fatalf("UploadFile() error = %v, want %v", e, wantErr)
	}

	result, err := client.DownloadFile(ctx, key)
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	defer func() { _ = result.Reader.Close() }()
	data, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("content = %q, want %q (failed overwrite destroyed the key)", data, "original")
	}
}

func TestClient_AtomicWriteLeavesNoTempFiles(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	client, err := NewClient(Config{RootDir: root})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// One successful write and one failed write, both into the same directory.
	if _, e := client.UploadFile(ctx, strings.NewReader("ok"), "d/good.bin"); e != nil {
		t.Fatalf("upload: %v", e)
	}
	if _, e := client.UploadFile(ctx, &failingReader{err: errors.New("boom")}, "d/bad.bin"); e == nil {
		t.Fatal("UploadFile() error = nil, want failure")
	}

	entries, err := os.ReadDir(filepath.Join(root, "d"))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "good.bin" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory contents = %v, want only [good.bin]", names)
	}
}

// TestClient_ListFilesHidesTempFiles ensures an orphaned temporary file, such as
// one left by a process killed mid-write, is never reported as a stored key.
func TestClient_ListFilesHidesTempFiles(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	client, err := NewClient(Config{RootDir: root})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, e := client.UploadFile(ctx, strings.NewReader("ok"), "real.bin"); e != nil {
		t.Fatalf("upload: %v", e)
	}
	orphan := filepath.Join(root, tmpFilePrefix+"123456")
	if e := os.WriteFile(orphan, []byte("junk"), 0o600); e != nil {
		t.Fatalf("write orphan: %v", e)
	}

	keys, _, err := client.ListFiles(ctx, "", "", 10)
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	if len(keys) != 1 || keys[0] != "real.bin" {
		t.Fatalf("keys = %v, want [real.bin]", keys)
	}
}

// TestClient_UploadFilePermissions pins the resulting file mode, since writing
// through os.CreateTemp would otherwise silently produce 0o600 files.
func TestClient_UploadFilePermissions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	client, err := NewClient(Config{RootDir: root})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, e := client.UploadFile(ctx, strings.NewReader("a"), "new.bin"); e != nil {
		t.Fatalf("upload: %v", e)
	}
	stat, err := os.Stat(filepath.Join(root, "new.bin"))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if stat.Mode().Perm() != defaultFileMode {
		t.Fatalf("new file mode = %v, want %v", stat.Mode().Perm(), defaultFileMode)
	}

	// An overwrite must not reset permissions the operator chose.
	if e := os.Chmod(filepath.Join(root, "new.bin"), 0o600); e != nil {
		t.Fatalf("Chmod() error = %v", e)
	}
	if _, e := client.UploadFile(ctx, strings.NewReader("b"), "new.bin"); e != nil {
		t.Fatalf("overwrite: %v", e)
	}
	stat, err = os.Stat(filepath.Join(root, "new.bin"))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("overwritten file mode = %v, want %v", stat.Mode().Perm(), os.FileMode(0o600))
	}
}

func TestClient_EmptyKeyRejected(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, e := client.UploadFile(ctx, io.NopCloser(nil), ""); !errors.Is(e, storageerr.ErrFileNameInvalid) {
		t.Fatalf("UploadFile(\"\") error = %v, want %v", e, storageerr.ErrFileNameInvalid)
	}
	if _, e := client.IsFileExists(ctx, ""); !errors.Is(e, storageerr.ErrFileNameInvalid) {
		t.Fatalf("IsFileExists(\"\") error = %v, want %v", e, storageerr.ErrFileNameInvalid)
	}
	if _, e := client.DownloadFile(ctx, ""); !errors.Is(e, storageerr.ErrFileNameInvalid) {
		t.Fatalf("DownloadFile(\"\") error = %v, want %v", e, storageerr.ErrFileNameInvalid)
	}
	if e := client.DeleteFile(ctx, ""); !errors.Is(e, storageerr.ErrFileNameInvalid) {
		t.Fatalf("DeleteFile(\"\") error = %v, want %v", e, storageerr.ErrFileNameInvalid)
	}
	if e := client.MoveFile(ctx, "", "dst.txt", true); !errors.Is(e, storageerr.ErrFileNameInvalid) {
		t.Fatalf("MoveFile(\"\") error = %v, want %v", e, storageerr.ErrFileNameInvalid)
	}
	if e := client.CopyFile(ctx, "", "dst.txt", true); !errors.Is(e, storageerr.ErrFileNameInvalid) {
		t.Fatalf("CopyFile(\"\") error = %v, want %v", e, storageerr.ErrFileNameInvalid)
	}
}

func TestClient_DirectoryKeyTreatedAsNotFound(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	client, err := NewClient(Config{RootDir: root})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Create a subdirectory that collides with a would-be key.
	if e := os.MkdirAll(filepath.Join(root, "images"), 0o750); e != nil {
		t.Fatalf("mkdir: %v", e)
	}

	exists, e := client.IsFileExists(ctx, "images")
	if e != nil {
		t.Fatalf("IsFileExists() error = %v", e)
	}
	if exists {
		t.Fatal("IsFileExists(dir) = true, want false")
	}

	if _, e = client.DownloadFile(ctx, "images"); !errors.Is(e, storageerr.ErrNotFound) {
		t.Fatalf("DownloadFile(dir) error = %v, want %v", e, storageerr.ErrNotFound)
	}
	// Deletion is idempotent, so a key holding a directory reports success; the
	// directory itself must still be left alone (asserted at the end).
	if e = client.DeleteFile(ctx, "images"); e != nil {
		t.Fatalf("DeleteFile(dir) error = %v, want nil", e)
	}
	if e = client.MoveFile(ctx, "images", "moved", true); !errors.Is(e, storageerr.ErrNotFound) {
		t.Fatalf("MoveFile(dir) error = %v, want %v", e, storageerr.ErrNotFound)
	}
	if e = client.CopyFile(ctx, "images", "copied", true); !errors.Is(e, storageerr.ErrNotFound) {
		t.Fatalf("CopyFile(dir) error = %v, want %v", e, storageerr.ErrNotFound)
	}

	// The directory must survive the refused operations.
	if info, statErr := os.Stat(filepath.Join(root, "images")); statErr != nil || !info.IsDir() {
		t.Fatalf("directory was modified: err=%v", statErr)
	}
}

func TestClient_CopyMissingSourcePreservesDestination(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	const destination = "release/live.txt"
	if _, err := client.UploadFile(ctx, strings.NewReader("current"), destination); err != nil {
		t.Fatalf("UploadFile(destination): %v", err)
	}

	err = client.CopyFile(ctx, "release/missing.txt", destination, true)
	if !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("CopyFile() error = %v, want %v", err, storageerr.ErrNotFound)
	}

	result, err := client.DownloadFile(ctx, destination)
	if err != nil {
		t.Fatalf("DownloadFile(destination): %v", err)
	}
	defer func() { _ = result.Reader.Close() }()
	got, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("ReadAll(destination): %v", err)
	}
	if string(got) != "current" {
		t.Fatalf("destination content = %q, want %q", got, "current")
	}
}
