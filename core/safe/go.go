package safe

import (
	"runtime/debug"

	"github.com/go-sphere/sphere/log"
)

// LogRecovered reports a recovered panic value r at error level through the
// package-level logger of github.com/go-sphere/sphere/log, with the fields
// module, error, and stack (the current goroutine's stack). It is the single
// panic-to-log entry point shared across the core packages so that the field
// names stay consistent.
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

// Go runs fn in a new goroutine with Recover and returns immediately. A panic
// in fn is logged, not re-raised. Go does not wait for fn; callers that need
// completion must synchronize on their own (for example a channel or
// sync.WaitGroup).
func Go(fn func()) {
	go Run(fn)
}

// Run calls fn with Recover on the current goroutine. It returns when fn
// returns or panics; a panic is logged and swallowed, so the caller cannot
// tell the two apart from Run itself.
func Run(fn func()) {
	defer Recover()
	fn()
}
