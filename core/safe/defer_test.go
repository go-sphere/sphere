package safe

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIfErrorPresent(t *testing.T) {
	t.Run("nil error does not invoke handler", func(t *testing.T) {
		var called atomic.Bool
		InitErrorHandler(func(err error) {
			called.Store(true)
		})
		defer InitErrorHandler(defaultErrorHandler)

		IfErrorPresent(func() error {
			return nil
		})

		if called.Load() {
			t.Fatal("handler should not be called on nil error")
		}
	})

	t.Run("non-nil error invokes active handler", func(t *testing.T) {
		var captured error
		expectedErr := errors.New("defer failed")
		InitErrorHandler(func(err error) {
			captured = err
		})
		defer InitErrorHandler(defaultErrorHandler)

		IfErrorPresent(func() error {
			return expectedErr
		})

		if !errors.Is(captured, expectedErr) {
			t.Fatalf("expected error %v, got %v", expectedErr, captured)
		}
	})
}

func TestIfErrorXPresent(t *testing.T) {
	t.Run("nil error does not invoke handler", func(t *testing.T) {
		var called atomic.Bool
		InitErrorHandler(func(err error) {
			called.Store(true)
		})
		defer InitErrorHandler(defaultErrorHandler)

		IfErrorXPresent(func() (int, error) {
			return 42, nil
		})

		if called.Load() {
			t.Fatal("handler should not be called on nil error")
		}
	})

	t.Run("non-nil error discards value and invokes handler", func(t *testing.T) {
		var captured error
		expectedErr := errors.New("read failed")
		InitErrorHandler(func(err error) {
			captured = err
		})
		defer InitErrorHandler(defaultErrorHandler)

		IfErrorXPresent(func() (string, error) {
			return "ignored_value", expectedErr
		})

		if !errors.Is(captured, expectedErr) {
			t.Fatalf("expected error %v, got %v", expectedErr, captured)
		}
	})
}

func TestInitErrorHandler_NilIgnored(t *testing.T) {
	var captured error
	expectedErr := errors.New("custom error")
	InitErrorHandler(func(err error) {
		captured = err
	})
	defer InitErrorHandler(defaultErrorHandler)

	// Passing nil must not clear the existing handler
	InitErrorHandler(nil)

	IfErrorPresent(func() error {
		return expectedErr
	})

	if !errors.Is(captured, expectedErr) {
		t.Fatalf("expected error %v after InitErrorHandler(nil), got %v", expectedErr, captured)
	}
}

func TestConcurrentInitAndIfErrorPresent(t *testing.T) {
	defer InitErrorHandler(defaultErrorHandler)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			InitErrorHandler(func(err error) {})
		}()
		go func() {
			defer wg.Done()
			IfErrorPresent(func() error {
				return errors.New("concurrent err")
			})
			IfErrorXPresent(func() (int, error) {
				return 1, errors.New("concurrent err x")
			})
		}()
	}
	wg.Wait()
}

// TestSafeStress_ConcurrentInitAndIfError verifies thread safety of InitErrorHandler
// and IfErrorPresent / IfErrorXPresent under heavy concurrent execution.
func TestSafeStress_ConcurrentInitAndIfError(t *testing.T) {
	var errCounter atomic.Int64
	defer InitErrorHandler(defaultErrorHandler)

	var wg sync.WaitGroup
	stopSignal := make(chan struct{})

	// 10 updater goroutines
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stopSignal:
					return
				default:
					InitErrorHandler(func(err error) {
						errCounter.Add(1)
					})
					// Also test nil ignore
					InitErrorHandler(nil)
				}
			}
		}(i)
	}

	// 20 reporter goroutines
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 200 {
				if j%2 == 0 {
					IfErrorPresent(func() error {
						return errors.New("err")
					})
				} else {
					IfErrorXPresent(func() (int, error) {
						return j, errors.New("err x")
					})
				}
			}
		}(i)
	}

	// Let reporter routines finish
	time.Sleep(50 * time.Millisecond)
	close(stopSignal)
	wg.Wait()

	if errCounter.Load() == 0 {
		t.Fatal("expected error handler to be invoked at least once")
	}
}
