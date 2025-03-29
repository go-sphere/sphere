package safe

import "log"

type ErrorHandler func(string, error)

var defaultErrorHandler ErrorHandler = func(label string, err error) {
	log.Printf("%s: %v", label, err)
}

func InitErrorHandler(handler ErrorHandler) {
	if handler != nil {
		defaultErrorHandler = handler
	}
}

func ErrorIfPresent(label string, fn func() error) {
	err := fn()
	if err != nil {
		defaultErrorHandler(label, err)
	}
}

func ErrorIfPresentWithValue[T any](label string, fn func() (T, error)) {
	_, err := fn()
	if err != nil {
		defaultErrorHandler(label, err)
		return
	}
}
