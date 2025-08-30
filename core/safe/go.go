package safe

import (
	"github.com/go-sphere/sphere/log"
)

// Recover handles panics and logs them with optional custom error handlers.
// It should be used in defer statements to catch and handle panics gracefully.
// Optional onError functions are called with the panic value for custom handling.
func Recover(onError ...func(err any)) {
	if r := recover(); r != nil {
		log.Error(
			"goroutine panic",
			log.String("module", "safe"),
			log.Any("error", r),
		)
		for _, fn := range onError {
			fn(r)
		}
	}
}

// Go starts a new goroutine that runs the provided function with panic recovery.
// Any panic in the function will be caught and logged without crashing the program.
func Go(fn func()) {
	go Run(fn)
}

// Run executes a function with panic recovery protection.
// It can be used to wrap potentially panicking code with automatic recovery.
func Run(fn func()) {
	defer Recover()
	fn()
}
