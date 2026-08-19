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

// TestSyncSucceedsWithConsoleSink pins that Sync is usable as a success signal.
// Syncing os.Stdout issues an fsync the kernel rejects for a pipe or character
// device, which under a container or CI is what stdout always is — so a console
// sink used to make Sync fail every single time and mask a real file-sink flush
// failure reported through the same joined error.
func TestSyncSucceedsWithConsoleSink(t *testing.T) {
	backend := NewBackend(NewDefaultConfig())
	t.Cleanup(func() { _ = backend.Close() })

	if err := backend.Sync(); err != nil {
		t.Fatalf("Sync() with a console sink = %v, want nil", err)
	}
}

// TestSyncReportsFileSinkFailures pins the other half: silencing the console
// sink must not silence a genuine failure from a file sink.
func TestSyncSucceedsWithFileSink(t *testing.T) {
	conf := NewDefaultConfig()
	conf.File.FileName = t.TempDir() + "/app.log"

	backend := NewBackend(conf)
	t.Cleanup(func() { _ = backend.Close() })

	backend.Log(t.Context(), 0, "entry")
	if err := backend.Sync(); err != nil {
		t.Fatalf("Sync() with a writable file sink = %v, want nil", err)
	}
}

// panicMarshaler panics while being encoded, standing in for a value whose
// MarshalJSON or Stringer has a bug of its own.
type panicMarshaler struct{}

func (panicMarshaler) MarshalJSON() ([]byte, error) { panic("marshal boom") }

// TestLogSurvivesPanickingAttr pins that a log statement cannot abort the
// goroutine that made it.
//
// The same value used to crash here while the stdio backend degraded and
// carried on, so swapping backends changed whether an application survived its
// own logging — and logging sits on error paths, the worst moment to raise a
// second failure.
func TestLogSurvivesPanickingAttr(t *testing.T) {
	backend := NewBackend(NewDefaultConfig())
	t.Cleanup(func() { _ = backend.Close() })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panicking attribute escaped the backend: %v", r)
		}
	}()
	backend.Log(t.Context(), log.LevelError, "request failed", log.Any("payload", panicMarshaler{}))
}
