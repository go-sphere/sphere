// Package multierr is a concurrent-safe error collector used by task.Group
// and task.Manager.
//
// The zero value of [Error] retains every error. Set [Error.Limit] before the
// first Add on a long-lived collector (Manager uses 1024). [Error.Errors]
// returns the joined error string, not a []error. [Error.Unwrap] returns
// errors.Join of the retained batch (plus a synthetic "N earlier errors
// dropped" when Limit discarded older ones), so errors.Is still matches
// members.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/core/task/multierr"
//
//	var errs multierr.Error
//	errs.Add(err1) // nil errors are ignored
//	errs.Add(err2)
//	if err := errs.Unwrap(); err != nil {
//		return err // errors.Is(err, err1) still matches
//	}
package multierr
