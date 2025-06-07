package task

import (
	"errors"
	"sync"
)

type ErrCollection struct {
	mu   sync.RWMutex
	errs []error
}

func (e *ErrCollection) Add(err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errs = append(e.errs, err)
}

func (e *ErrCollection) Err() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.errs) == 0 {
		return nil
	}
	return errors.Join(e.errs...)
}
