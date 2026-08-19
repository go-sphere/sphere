package log

import (
	"context"
	"errors"
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
