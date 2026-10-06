package multierr_test

import (
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/core/task/multierr"
)

var errNotFound = errors.New("not found")

func ExampleError() {
	var errs multierr.Error
	errs.Add(nil) // ignored
	errs.Add(errNotFound)
	errs.Add(errors.New("timeout"))

	err := errs.Unwrap()
	fmt.Println(errors.Is(err, errNotFound))
	fmt.Println(errs.Errors())
	// Output:
	// true
	// not found
	// timeout
}

// A bounded collector keeps the newest Limit errors and reports the loss.
func ExampleError_limit() {
	errs := multierr.Error{Limit: 2}
	for i := range 4 {
		errs.Add(fmt.Errorf("attempt %d failed", i))
	}
	fmt.Println(errs.Errors())
	// Output:
	// 2 earlier errors dropped
	// attempt 2 failed
	// attempt 3 failed
}
