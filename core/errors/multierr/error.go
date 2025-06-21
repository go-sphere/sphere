package multierr

import (
	"errors"
	"sync"
)

type Error struct {
	mu   sync.RWMutex
	errs []error
}

func (e *Error) Add(err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errs = append(e.errs, err)
}

func (e *Error) Errors() string {
	return e.Unwrap().Error()
}

func (e *Error) Unwrap() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.errs) == 0 {
		return nil
	}
	return errors.Join(e.errs...)
}
