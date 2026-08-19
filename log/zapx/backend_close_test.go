package zapx

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-sphere/sphere/log"
)

func newFileBackend(t *testing.T) *Backend {
	t.Helper()
	conf := NewDefaultConfig()
	conf.Console.Disable = true
	conf.File.FileName = filepath.Join(t.TempDir(), "app.log")
	return NewBackend(conf)
}

func TestBackendCloseReleasesFile(t *testing.T) {
	backend := newFileBackend(t)
	backend.Log(context.Background(), log.LevelInfo, "before close")

	if err := backend.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Close releases the handle; it does not seal the backend. lumberjack reopens
	// on the next write, so a double Close must stay harmless.
	if err := backend.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestDerivedBackendCloseIsNoop pins the ownership rule: only the backend that
// opened the writer closes it, so a logger derived through With can never
// release its parent's handle.
func TestDerivedBackendCloseIsNoop(t *testing.T) {
	parent := newFileBackend(t)
	t.Cleanup(func() { _ = parent.Close() })

	derived, ok := parent.With(log.WithAttrs(map[string]any{"scope": "child"})).(*Backend)
	if !ok {
		t.Fatal("With should return a *Backend")
	}
	if derived.file != nil {
		t.Fatal("derived backend must not own the rotating file writer")
	}
	if err := derived.Close(); err != nil {
		t.Fatalf("derived Close: %v", err)
	}
	if parent.file == nil {
		t.Fatal("parent lost ownership of its writer")
	}
}

func TestBackendWithoutFileCloseIsNoop(t *testing.T) {
	backend := NewBackend(NewDefaultConfig())
	if backend.file != nil {
		t.Fatal("a console-only backend must not own a writer")
	}
	if err := backend.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
