package log

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStderr mirrors captureStdout for entries the backend routes to stderr
// (LevelError and above).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}

// TestStdioBackendStackAtAttachesStack pins that WithStackAt actually emits a
// stack trace at and above the configured level, and nothing below it.
func TestStdioBackendStackAtAttachesStack(t *testing.T) {
	b := NewStdioBackend(WithStackAt(LevelError))

	out := captureStdout(t, func() {
		b.Log(context.Background(), LevelInfo, "no stack here")
	})
	if strings.Contains(out, "stack=") {
		t.Errorf("entry below the stack level should not carry a stack, got: %q", out)
	}

	errOut := captureStderr(t, func() {
		b.Log(context.Background(), LevelError, "stack expected")
	})
	if !strings.Contains(errOut, "stack=") {
		t.Errorf("entry at the stack level should carry a stack, got: %q", errOut)
	}
	if !strings.Contains(errOut, "backend_stdio_stack_test.go") {
		t.Errorf("stack should reference the calling site, got: %q", errOut)
	}
}

// TestStdioBackendStackAtDoesNotMutateCallerAttrs pins that attaching the stack
// does not write into a slice the caller may reuse.
func TestStdioBackendStackAtDoesNotMutateCallerAttrs(t *testing.T) {
	b := NewStdioBackend(WithStackAt(LevelError))
	attrs := make([]Attr, 1, 8)
	attrs[0] = String("key", "value")

	_ = captureStderr(t, func() {
		b.Log(context.Background(), LevelError, "msg", attrs...)
	})

	if len(attrs) != 1 {
		t.Fatalf("caller slice length changed to %d", len(attrs))
	}
	if attrs[0].Key != "key" {
		t.Errorf("caller slice was mutated: %+v", attrs[0])
	}
}
