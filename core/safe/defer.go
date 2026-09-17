package safe

import (
	"sync/atomic"

	"github.com/go-sphere/sphere/log"
)

// ErrorHandler handles an error reported by IfErrorPresent or IfErrorXPresent.
// It takes only the error; there is no label argument.
//
// The swap of the active handler is atomic, but the handler itself is not: it
// can be running in any number of goroutines at once (including the previous
// handler, which may still be in flight), so an implementation must be safe for
// concurrent use and must not mutate captured state unguarded.
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

// InitErrorHandler replaces the handler used by IfErrorPresent and
// IfErrorXPresent. A nil handler is ignored. It does not affect Recover.
func InitErrorHandler(handler ErrorHandler) {
	if handler != nil {
		errorHandler.Store(&handler)
	}
}

// IfErrorPresent calls fn and, if it returns a non-nil error, passes that
// error to the handler set by InitErrorHandler.
func IfErrorPresent(fn func() error) {
	err := fn()
	if err != nil {
		(*errorHandler.Load())(err)
	}
}

// IfErrorXPresent calls fn and discards the value on both success and failure.
// A non-nil error is passed to the handler set by InitErrorHandler.
func IfErrorXPresent[T any](fn func() (T, error)) {
	_, err := fn()
	if err != nil {
		(*errorHandler.Load())(err)
		return
	}
}
