// Package safe is panic recovery and deferred-error reporting used by boot
// and task. Library code should use Go or Run instead of a bare go func().
//
// Go and Run never re-panic: they log via LogRecovered then continue.
// Recover is for defer. InitErrorHandler replaces the handler used by
// IfErrorPresent / IfErrorXPresent (signature func(error), no label); it is
// unrelated to Recover's onError callbacks.
package safe

import (
	"runtime/debug"

	"github.com/go-sphere/sphere/log"
)

// LogRecovered reports a recovered panic value using a unified structured
// format. It is the single panic-to-log entry point shared across the core
// packages so that the field names (module, error, stack) stay consistent.
func LogRecovered(module string, r any) {
	log.Error(
		"panic recovered",
		log.String("module", module),
		log.Any("error", r),
		log.String("stack", string(debug.Stack())),
	)
}

// Recover must be deferred. It logs the panic via LogRecovered, then calls
// each onError with the panic value. It does not re-panic. onError is not
// the ErrorHandler used by IfErrorPresent.
func Recover(onError ...func(err any)) {
	if r := recover(); r != nil {
		LogRecovered("safe", r)
		for _, fn := range onError {
			fn(r)
		}
	}
}

// Go runs fn in a new goroutine with Recover. A panic is logged, not re-raised.
func Go(fn func()) {
	go Run(fn)
}

// Run calls fn with Recover on the current goroutine.
func Run(fn func()) {
	defer Recover()
	fn()
}
