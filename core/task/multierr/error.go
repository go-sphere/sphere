// Package multierr is a concurrent-safe error collector used by task.Group
// and task.Manager.
//
// The zero value retains every error. Set Limit before the first Add on a
// long-lived collector (Manager uses 1024). Errors returns the joined error
// string, not a []error. Unwrap returns errors.Join of the retained batch
// (plus a synthetic "N earlier errors dropped" when Limit discarded older
// ones), so errors.Is still matches members.
package multierr

import (
	"errors"
	"fmt"
	"sync"
)

// Error collects errors from concurrent producers. The zero value is ready to
// use and retains everything added to it.
type Error struct {
	// Limit caps how many errors are retained; the oldest are dropped once it is
	// exceeded, and Unwrap reports how many were lost. A non-positive value (the
	// zero value) retains everything, which is what per-call collectors want
	// since they are already bounded by the number of tasks. Set it on collectors
	// that live as long as the process, where an unbounded slice would grow with
	// every failure. Assign Limit before the first Add; it is not safe to change
	// concurrently with Add.
	Limit int

	mu      sync.RWMutex
	errs    []error
	dropped int
}

// Add appends err. A nil err is ignored.
func (e *Error) Add(err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errs = append(e.errs, err)
	if e.Limit <= 0 || len(e.errs) <= e.Limit {
		return
	}
	// Compact in place so the backing array stays bounded instead of creeping
	// forward one slot per drop.
	drop := len(e.errs) - e.Limit
	e.errs = append(e.errs[:0], e.errs[drop:]...)
	e.dropped += drop
}

// Errors returns Unwrap().Error(), or "" when nothing has been added.
func (e *Error) Errors() string {
	err := e.Unwrap()
	if err == nil {
		return ""
	}
	return err.Error()
}

// Unwrap returns errors.Join of the retained errors. When Limit dropped
// earlier entries, the join starts with a synthetic "N earlier errors dropped".
func (e *Error) Unwrap() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.errs) == 0 {
		return nil
	}
	if e.dropped == 0 {
		return errors.Join(e.errs...)
	}
	// Surface the loss rather than silently returning a truncated set.
	joined := make([]error, 0, len(e.errs)+1)
	joined = append(joined, fmt.Errorf("%d earlier errors dropped", e.dropped))
	joined = append(joined, e.errs...)
	return errors.Join(joined...)
}
