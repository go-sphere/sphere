package log

import (
	"context"
	"errors"
	"io"
	"testing"
)

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

// TestInitWithBackendsDoesNotCloseOldBackend pins the ownership contract: the
// caller constructs backends, so a swap must leave the previous one usable.
func TestInitWithBackendsDoesNotCloseOldBackend(t *testing.T) {
	original := logger()
	t.Cleanup(func() { std.Store(original) })

	old := &closableBackend{}
	InitWithBackends(old)
	InitWithBackends(&closableBackend{})

	if old.closed != 0 {
		t.Errorf("previous backend was closed %d times, want 0", old.closed)
	}
	// Still usable by whoever kept the reference.
	old.Log(context.Background(), LevelInfo, "still alive")
}

// TestInitWithBackendsKeepsCurrentOnEmpty pins that a configuration mistake
// cannot silence the process. InitWithBackends returns nothing, so a caller
// whose backend builder returned nil on one branch had no way to notice that
// every later log line was being discarded — and that branch is exactly the one
// taken when the configuration is already wrong.
func TestInitWithBackendsKeepsCurrentOnEmpty(t *testing.T) {
	restore := &captureBackend{}
	InitWithBackends(restore)

	for _, tc := range []struct {
		name     string
		backends []Backend
	}{
		{name: "no arguments"},
		{name: "single nil", backends: []Backend{nil}},
		{name: "all nil", backends: []Backend{nil, nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			InitWithBackends(tc.backends...)

			before := len(restore.entries)
			Error("still logging")
			if len(restore.entries) != before+1 {
				t.Fatal("the installed backend was replaced by a silent one")
			}
		})
	}
}

// TestInitWithBackendsAcceptsExplicitNop pins the escape hatch: discarding logs
// on purpose must still work, and must not be mistaken for the empty case.
func TestInitWithBackendsAcceptsExplicitNop(t *testing.T) {
	seen := &captureBackend{}
	InitWithBackends(seen)
	InitWithBackends(NewNopBackend())

	before := len(seen.entries)
	Error("discarded")
	if len(seen.entries) != before {
		t.Fatal("NewNopBackend() must replace the current backend")
	}
}

// TestContextMergeBackendForwardsClose pins that the recommended wrapper does
// not swallow Close. The documented release pattern type-asserts the installed
// backend to io.Closer; a wrapper that does not implement it silently skips the
// release and leaks the file handle underneath.
func TestContextMergeBackendForwardsClose(t *testing.T) {
	inner := &captureBackend{}
	wrapped := WrapBackendWithContextMerge(inner, func(context.Context) []Attr { return nil })

	closer, ok := wrapped.(io.Closer)
	if !ok {
		t.Fatal("context merge backend must implement io.Closer so wrapped handles can be released")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if !inner.closed {
		t.Fatal("Close() must reach the wrapped backend")
	}
}

type captureBackend struct {
	entries []string
	closed  bool
}

func (c *captureBackend) Log(_ context.Context, _ Level, msg string, _ ...Attr) {
	c.entries = append(c.entries, msg)
}

func (c *captureBackend) Sync() error { return nil }

func (c *captureBackend) With(...Option) Backend { return c }

func (c *captureBackend) Close() error {
	c.closed = true
	return nil
}
