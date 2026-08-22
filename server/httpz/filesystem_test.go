package httpz

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestFs pins the source priority: an existing local directory wins over an
// embedded filesystem, a missing one falls back to it, and a config that names
// neither is an error rather than an empty FS.
func TestFs(t *testing.T) {
	t.Run("local directory wins", func(t *testing.T) {
		dir := t.TempDir()
		got, err := Fs(dir, fstest.MapFS{}, "sub")
		if err != nil {
			t.Fatalf("Fs: %v", err)
		}
		if _, err := fs.Stat(got, "."); err != nil {
			t.Fatalf("the returned FS does not serve the local dir: %v", err)
		}
	})

	t.Run("fallback to embedded when local is missing", func(t *testing.T) {
		embedded := fstest.MapFS{"sub/a.txt": {Data: []byte("a")}}
		got, err := Fs(t.TempDir()+"/does-not-exist", embedded, "sub")
		if err != nil {
			t.Fatalf("Fs: %v", err)
		}
		if _, err := fs.Stat(got, "a.txt"); err != nil {
			t.Fatalf("the embedded subtree was not served: %v", err)
		}
	})

	t.Run("local path that is a file does not win", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "plain.txt")
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		embedded := fstest.MapFS{"sub/a.txt": {Data: []byte("a")}}
		got, err := Fs(file, embedded, "sub")
		if err != nil {
			t.Fatalf("Fs: %v", err)
		}
		if _, err := fs.Stat(got, "a.txt"); err != nil {
			t.Fatalf("expected the embedded fallback, got: %v", err)
		}
	})

	t.Run("no source is an error", func(t *testing.T) {
		if _, err := Fs("", nil, ""); err == nil {
			t.Fatal("Fs() = nil error, want an explicit failure")
		}
	})
}
