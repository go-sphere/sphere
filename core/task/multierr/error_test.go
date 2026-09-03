package multierr

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestZeroValueRetainsEverything(t *testing.T) {
	var e Error
	for i := range 100 {
		e.Add(fmt.Errorf("err-%d", i))
	}
	if got := len(e.errs); got != 100 {
		t.Fatalf("retained %d errors, want 100", got)
	}
	if e.dropped != 0 {
		t.Errorf("dropped %d errors, want 0 for an uncapped collector", e.dropped)
	}
}

func TestLimitDropsOldestAndReportsLoss(t *testing.T) {
	e := Error{Limit: 3}
	first := errors.New("err-0")
	for i := range 10 {
		if i == 0 {
			e.Add(first)
			continue
		}
		e.Add(fmt.Errorf("err-%d", i))
	}

	if got := len(e.errs); got != 3 {
		t.Fatalf("retained %d errors, want 3", got)
	}
	if e.dropped != 7 {
		t.Fatalf("dropped %d errors, want 7", e.dropped)
	}

	joined := e.Unwrap()
	if joined == nil {
		t.Fatal("Unwrap returned nil despite retained errors")
	}
	if errors.Is(joined, first) {
		t.Error("the oldest error should have been dropped")
	}
	msg := joined.Error()
	if !strings.Contains(msg, "7 earlier errors dropped") {
		t.Errorf("Unwrap should report the loss, got %q", msg)
	}
	// The newest errors must survive.
	for _, want := range []string{"err-7", "err-8", "err-9"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Unwrap lost %q, got %q", want, msg)
		}
	}
}

func TestLimitClearsDroppedReferences(t *testing.T) {
	e := Error{Limit: 2, errs: make([]error, 0, 3)}
	e.Add(errors.New("err-0"))
	e.Add(errors.New("err-1"))
	e.Add(errors.New("err-2"))

	backing := e.errs[:cap(e.errs)]
	if backing[len(e.errs)] != nil {
		t.Fatalf("dropped backing slot still retains %v", backing[len(e.errs)])
	}
}

// TestLimitKeepsBackingArrayBounded pins that repeated eviction compacts in
// place instead of letting the backing array creep forward one slot per drop.
func TestLimitKeepsBackingArrayBounded(t *testing.T) {
	e := Error{Limit: 4}
	for i := range 10_000 {
		e.Add(fmt.Errorf("err-%d", i))
	}
	if got := cap(e.errs); got > 16 {
		t.Errorf("backing array grew to cap %d, want it to stay near the limit", got)
	}
}

func TestAddNilIgnored(t *testing.T) {
	var e Error
	e.Add(nil)
	if len(e.errs) != 0 {
		t.Fatalf("expected 0 errors after Add(nil), got %d", len(e.errs))
	}
	if e.Unwrap() != nil {
		t.Fatalf("expected Unwrap() to be nil, got %v", e.Unwrap())
	}
	if e.Errors() != "" {
		t.Fatalf("expected Errors() to be empty string, got %q", e.Errors())
	}
}

func TestEmptyUnwrapAndErrors(t *testing.T) {
	var e Error
	if err := e.Unwrap(); err != nil {
		t.Fatalf("expected nil from empty Unwrap, got %v", err)
	}
	if msg := e.Errors(); msg != "" {
		t.Fatalf("expected empty string from empty Errors, got %q", msg)
	}
}

func TestErrorsString(t *testing.T) {
	var e Error
	e.Add(errors.New("error alpha"))
	e.Add(errors.New("error beta"))

	msg := e.Errors()
	if !strings.Contains(msg, "error alpha") || !strings.Contains(msg, "error beta") {
		t.Fatalf("Errors() missing expected messages: %q", msg)
	}
	if msg != e.Unwrap().Error() {
		t.Fatalf("Errors() %q != Unwrap().Error() %q", msg, e.Unwrap().Error())
	}
}
