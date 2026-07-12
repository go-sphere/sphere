package log

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout while fn runs and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}

func TestStdioBackendMinLevel(t *testing.T) {
	b := NewStdioBackend(WithMinLevel(LevelWarn))
	out := captureStdout(t, func() {
		b.Log(context.Background(), LevelDebug, "debug msg")
		b.Log(context.Background(), LevelInfo, "info msg")
		b.Log(context.Background(), LevelWarn, "warn msg")
	})
	if strings.Contains(out, "debug msg") || strings.Contains(out, "info msg") {
		t.Fatalf("entries below MinLevel should be discarded, got: %q", out)
	}
	if !strings.Contains(out, "warn msg") {
		t.Fatalf("entries at or above MinLevel should be emitted, got: %q", out)
	}
}

func TestStdioBackendStackAtDoesNotFilter(t *testing.T) {
	// WithStackAt must not act as a level filter (BUG-16).
	b := NewStdioBackend(WithStackAt(LevelError))
	out := captureStdout(t, func() {
		b.Log(context.Background(), LevelInfo, "info msg")
	})
	if !strings.Contains(out, "info msg") {
		t.Fatalf("WithStackAt must not filter out lower-level entries, got: %q", out)
	}
}

func TestStdioBackendAddCaller(t *testing.T) {
	b := NewStdioBackend(AddCaller())
	out := captureStdout(t, func() {
		b.Log(context.Background(), LevelInfo, "with caller")
	})
	if !strings.Contains(out, "caller=") {
		t.Fatalf("AddCaller should emit a caller field, got: %q", out)
	}

	b2 := NewStdioBackend()
	out2 := captureStdout(t, func() {
		b2.Log(context.Background(), LevelInfo, "no caller")
	})
	if strings.Contains(out2, "caller=") {
		t.Fatalf("caller field should be absent by default, got: %q", out2)
	}
}
