package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestRecoverHandlerPreservesErrorPanic pins that a handler panicking with an
// error value keeps that error's identity in the returned error, so callers can
// still match it with errors.Is/As.
func TestRecoverHandlerPreservesErrorPanic(t *testing.T) {
	sentinel := errors.New("boom")
	err := RecoverHandler(func(context.Context) error { panic(sentinel) })(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("errors.Is(err, sentinel) = false, err = %v", err)
	}
	if !strings.Contains(err.Error(), "scheduler: handler panic: boom") {
		t.Fatalf("error text = %q", err.Error())
	}
}

// TestRecoverPayloadHandlerRendersNonErrorPanic pins the fallback for panic
// values that are not errors.
func TestRecoverPayloadHandlerRendersNonErrorPanic(t *testing.T) {
	err := RecoverPayloadHandler(func(context.Context, []byte) error { panic("boom") })(context.Background(), nil)
	if err == nil || err.Error() != "scheduler: handler panic: boom" {
		t.Fatalf("err = %v", err)
	}
}
