package safe

import (
	"sync/atomic"

	"github.com/go-sphere/sphere/log"
)

// ErrorHandler defines a function type for handling errors with contextual labels.
// It takes a descriptive label and the error that occurred.
type ErrorHandler func(error)

// defaultErrorHandler reports errors through the framework logger.
func defaultErrorHandler(err error) {
	log.Error("safe: deferred error", log.Err(err))
}

// errorHandler holds the active error handler. It is swapped atomically so that
// InitErrorHandler can be called concurrently with IfErrorPresent/IfErrorXPresent.
var errorHandler atomic.Pointer[ErrorHandler]

func init() {
	var h ErrorHandler = defaultErrorHandler
	errorHandler.Store(&h)
}

// InitErrorHandler sets a custom error handler for the safe package.
// If handler is nil, the default error handler remains unchanged.
func InitErrorHandler(handler ErrorHandler) {
	if handler != nil {
		errorHandler.Store(&handler)
	}
}

// IfErrorPresent executes the given function and handles any error using the configured error handler.
// This is useful for safely executing functions that may fail without disrupting the main flow.
func IfErrorPresent(fn func() error) {
	err := fn()
	if err != nil {
		(*errorHandler.Load())(err)
	}
}

// IfErrorXPresent executes the given function that returns a value and error, handling any error.
// The returned value is discarded, making this useful for operations where you only care about side effects.
func IfErrorXPresent[T any](fn func() (T, error)) {
	_, err := fn()
	if err != nil {
		(*errorHandler.Load())(err)
		return
	}
}
