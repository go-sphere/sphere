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
