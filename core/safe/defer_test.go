package safe

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
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

	var handledA atomic.Int64
	var handledB atomic.Int64
	handlerA := func(error) { handledA.Add(1) }
	handlerB := func(error) { handledB.Add(1) }
	InitErrorHandler(handlerA)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for updater := range 4 {
		wg.Go(func() {
			<-start
			for i := range 100 {
				if (updater+i)%2 == 0 {
					InitErrorHandler(handlerA)
				} else {
					InitErrorHandler(handlerB)
				}
			}
		})
	}
	for range 8 {
		wg.Go(func() {
			<-start
			for range 100 {
				IfErrorPresent(func() error { return errors.New("concurrent err") })
			}
		})
	}
	close(start)
	wg.Wait()

	if got, want := handledA.Load()+handledB.Load(), int64(800); got != want {
		t.Fatalf("handled errors = %d, want %d", got, want)
	}
}
