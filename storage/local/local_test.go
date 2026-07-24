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
		if !errors.Is(fixErr, storageerr.ErrorFileNameInvalid) {
			t.Fatalf("fixFilePath() error = %v, want %v", fixErr, storageerr.ErrorFileNameInvalid)
		}
	})

	t.Run("reject empty and root keys", func(t *testing.T) {
		for _, key := range []string{"", ".", "/"} {
			_, fixErr := client.fixFilePath(key)
			if !errors.Is(fixErr, storageerr.ErrorFileNameInvalid) {
				t.Fatalf("fixFilePath(%q) error = %v, want %v", key, fixErr, storageerr.ErrorFileNameInvalid)
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

func TestClient_EmptyKeyRejected(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, e := client.UploadFile(ctx, io.NopCloser(nil), ""); !errors.Is(e, storageerr.ErrorFileNameInvalid) {
		t.Fatalf("UploadFile(\"\") error = %v, want %v", e, storageerr.ErrorFileNameInvalid)
	}
	if _, e := client.IsFileExists(ctx, ""); !errors.Is(e, storageerr.ErrorFileNameInvalid) {
		t.Fatalf("IsFileExists(\"\") error = %v, want %v", e, storageerr.ErrorFileNameInvalid)
	}
	if _, e := client.DownloadFile(ctx, ""); !errors.Is(e, storageerr.ErrorFileNameInvalid) {
		t.Fatalf("DownloadFile(\"\") error = %v, want %v", e, storageerr.ErrorFileNameInvalid)
	}
	if e := client.DeleteFile(ctx, ""); !errors.Is(e, storageerr.ErrorFileNameInvalid) {
		t.Fatalf("DeleteFile(\"\") error = %v, want %v", e, storageerr.ErrorFileNameInvalid)
	}
	if e := client.MoveFile(ctx, "", "dst.txt", true); !errors.Is(e, storageerr.ErrorFileNameInvalid) {
		t.Fatalf("MoveFile(\"\") error = %v, want %v", e, storageerr.ErrorFileNameInvalid)
	}
	if e := client.CopyFile(ctx, "", "dst.txt", true); !errors.Is(e, storageerr.ErrorFileNameInvalid) {
		t.Fatalf("CopyFile(\"\") error = %v, want %v", e, storageerr.ErrorFileNameInvalid)
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

	if _, e = client.DownloadFile(ctx, "images"); !errors.Is(e, storageerr.ErrorNotFound) {
		t.Fatalf("DownloadFile(dir) error = %v, want %v", e, storageerr.ErrorNotFound)
	}
	if e = client.DeleteFile(ctx, "images"); !errors.Is(e, storageerr.ErrorNotFound) {
		t.Fatalf("DeleteFile(dir) error = %v, want %v", e, storageerr.ErrorNotFound)
	}
	if e = client.MoveFile(ctx, "images", "moved", true); !errors.Is(e, storageerr.ErrorNotFound) {
		t.Fatalf("MoveFile(dir) error = %v, want %v", e, storageerr.ErrorNotFound)
	}
	if e = client.CopyFile(ctx, "images", "copied", true); !errors.Is(e, storageerr.ErrorNotFound) {
		t.Fatalf("CopyFile(dir) error = %v, want %v", e, storageerr.ErrorNotFound)
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
	if !errors.Is(err, storageerr.ErrorNotFound) {
		t.Fatalf("CopyFile() error = %v, want %v", err, storageerr.ErrorNotFound)
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
