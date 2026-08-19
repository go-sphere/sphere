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

// panicMarshaler panics while being formatted, standing in for a value whose
// Stringer or MarshalJSON has a bug of its own.
type panicMarshaler struct{}

func (panicMarshaler) String() string { panic("marshal boom") }

// TestLogSurvivesPanickingAttr pins that a log statement cannot abort the
// goroutine that made it. Attribute values are arbitrary caller data, and
// logging happens on error paths — the worst possible moment to raise a second
// failure. Backends must degrade to a diagnostic instead.
func TestLogSurvivesPanickingAttr(t *testing.T) {
	backend := &StdioBackend{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panicking attribute escaped the backend: %v", r)
		}
	}()
	backend.Log(context.Background(), LevelError, "request failed", Any("payload", panicMarshaler{}))
}

// TestLogSurvivesSelfReferentialValue pins the case that cannot be recovered at
// all: fmt.Sprint has no cycle detection, so a self-referential container
// recurses until the goroutine stack is exhausted, and a stack overflow is
// fatal to the whole process. It has to be prevented before formatting starts.
func TestLogSurvivesSelfReferentialValue(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic

	backend := &StdioBackend{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a cyclic attribute escaped the backend: %v", r)
		}
	}()
	backend.Log(context.Background(), LevelError, "cyclic", Any("payload", cyclic))
}

// TestFormatAnyKeepsOrdinaryValues pins that the cycle guard does not degrade
// values that are merely nested.
func TestFormatAnyKeepsOrdinaryValues(t *testing.T) {
	nested := map[string]any{
		"a": []int{1, 2, 3},
		"b": map[string]string{"k": "v"},
	}
	shared := map[string]string{"x": "y"}
	// The same value twice side by side is not a cycle.
	sideBySide := []any{shared, shared}

	for _, v := range []any{42, "text", nested, sideBySide} {
		if got := formatAny(v); strings.Contains(got, "unformattable") {
			t.Errorf("formatAny(%v) = %q, want the value formatted", v, got)
		}
	}
}
