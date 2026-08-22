package log

import (
	"context"
	"errors"
	"testing"
)

// TestMultiBackendFanOut pins that every child sees every entry, that a single
// child is passed through unwrapped, and that an all-nil list collapses to the
// no-op backend rather than panicking on the first log line.
func TestMultiBackendFanOut(t *testing.T) {
	a, b := &recordingBackend{}, &recordingBackend{}
	multi := NewMultiBackend(a, nil, b)

	multi.Log(context.Background(), LevelWarn, "fanned out", String("k", "v"))

	for name, child := range map[string]*recordingBackend{"first": a, "second": b} {
		got := child.last(t)
		if got.msg != "fanned out" || got.level != LevelWarn {
			t.Errorf("%s child got (%d, %q), want (%d, %q)", name, got.level, got.msg, LevelWarn, "fanned out")
		}
	}

	if got := NewMultiBackend(a); got != Backend(a) {
		t.Error("a single backend must be used directly rather than wrapped")
	}
	if got := NewMultiBackend(nil, nil); got == nil {
		t.Error("an unusable list must collapse to a no-op backend, not nil")
	} else {
		// Must be safe to use.
		got.Log(context.Background(), LevelError, "discarded")
	}
}

// TestMultiBackendSyncJoinsFailures pins that one child failing to flush neither
// hides the others' failures nor stops them from being attempted.
func TestMultiBackendSyncJoinsFailures(t *testing.T) {
	first := &recordingBackend{syncErr: context.Canceled}
	second := &recordingBackend{syncErr: context.DeadlineExceeded}
	multi := NewMultiBackend(first, second)

	err := multi.Sync()
	if err == nil {
		t.Fatal("Sync() = nil, want the children's failures")
	}
	for _, want := range []error{context.Canceled, context.DeadlineExceeded} {
		if !errors.Is(err, want) {
			t.Errorf("Sync() = %v, want it to report %v", err, want)
		}
	}
}

// TestNopBackendIsASafeDefault pins that the documented way to silence logging
// is genuinely inert: every operation must succeed and nothing may escape.
func TestNopBackendIsASafeDefault(t *testing.T) {
	nop := NewNopBackend()

	nop.Log(context.Background(), LevelError, "discarded", String("k", "v"))
	if err := nop.Sync(); err != nil {
		t.Fatalf("Sync() = %v, want nil", err)
	}
	derived := nop.With(WithName("x"))
	derived.Log(context.Background(), LevelInfo, "still discarded")
}

// closableBackend is a nop backend that records Close calls.
type closableBackend struct {
	nopBackend
	closed   int
	closeErr error
}

func (c *closableBackend) Close() error {
	c.closed++
	return c.closeErr
}

func TestMultiBackendCloseForwardsToChildren(t *testing.T) {
	first := &closableBackend{}
	second := &closableBackend{closeErr: errors.New("second failed")}
	// A child without Close must be skipped rather than aborting the sweep.
	plain := NewStdioBackend()

	multi, ok := NewMultiBackend(first, plain, second).(*MultiBackend)
	if !ok {
		t.Fatal("expected a *MultiBackend for three children")
	}

	err := multi.Close()
	if !errors.Is(err, second.closeErr) {
		t.Errorf("Close error = %v, want it to wrap %v", err, second.closeErr)
	}
	if first.closed != 1 {
		t.Errorf("first child closed %d times, want 1", first.closed)
	}
	if second.closed != 1 {
		t.Errorf("second child closed %d times, want 1", second.closed)
	}
}
