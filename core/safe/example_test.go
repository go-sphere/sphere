package safe_test

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-sphere/sphere/core/safe"
)

// A panic inside Run is logged and swallowed; the caller continues.
func ExampleRun() {
	safe.Run(func() {
		panic("boom")
	})
	fmt.Println("still running")
	// Output: still running
}

// Go does not wait for fn; synchronize on your own when completion matters.
func ExampleGo() {
	var wg sync.WaitGroup
	wg.Add(1)
	safe.Go(func() {
		defer wg.Done()
		fmt.Println("background work")
	})
	wg.Wait()
	// Output: background work
}

// Recover must be deferred directly. onError receives the panic value.
func ExampleRecover() {
	func() {
		defer safe.Recover(func(r any) {
			fmt.Println("recovered:", r)
		})
		panic("boom")
	}()
	// Output: recovered: boom
}

// IfErrorPresent forwards a deferred cleanup error to the handler installed
// with InitErrorHandler. The handler is process-wide; install it once at
// startup.
func ExampleIfErrorPresent() {
	safe.InitErrorHandler(func(err error) {
		fmt.Println("cleanup failed:", err)
	})

	closeFn := func() error { return errors.New("close: already closed") }
	func() {
		defer safe.IfErrorPresent(closeFn)
	}()
	// Output: cleanup failed: close: already closed
}
