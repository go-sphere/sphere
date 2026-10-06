// Package safe provides panic recovery and deferred-error reporting for
// library goroutines. It is used by core/boot and core/task; library code
// should use [Go] or [Run] instead of a bare go func().
//
// [Go], [Run], and a deferred [Recover] never re-panic: a recovered panic is
// logged through [LogRecovered] with the package-level logger of
// github.com/go-sphere/sphere/log, and execution continues.
// [IfErrorPresent] and [IfErrorXPresent] forward a non-nil error from a
// cleanup call (typically in a defer) to the process-wide [ErrorHandler]
// installed with [InitErrorHandler]; by default that handler logs the error.
// The ErrorHandler is unrelated to Recover's onError callbacks.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/core/safe"
//
//	safe.Go(func() {
//		// background work; a panic here is logged instead of crashing
//	})
//
//	f, err := os.Open(path)
//	if err != nil {
//		return err
//	}
//	defer safe.IfErrorPresent(f.Close) // a Close error goes to the ErrorHandler
//
// Call [InitErrorHandler] once during startup to route deferred errors
// elsewhere; it affects every later IfErrorPresent and IfErrorXPresent call
// in the process.
package safe
