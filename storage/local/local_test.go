package local

import (
	"errors"
	"path/filepath"
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
}
