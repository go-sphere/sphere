package safe

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/log"
)

// TestMain silences the global logger so the panic-recovery tests, which
// deliberately panic, do not spray stack traces over the test output.
func TestMain(m *testing.M) {
	log.InitWithBackends(log.NewNopBackend())
	m.Run()
}

// TestRunRecoversPanics pins the core promise of the package: a panic in the
// wrapped function is contained and the caller keeps running.
func TestRunRecoversPanics(t *testing.T) {
	ran := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic escaped Run: %v", r)
			}
		}()
		Run(func() {
			ran = true
			panic("boom")
		})
	}()
	if !ran {
		t.Fatal("the wrapped function did not run")
	}
}

// TestRunWithoutPanic pins that wrapping a clean function changes nothing.
func TestRunWithoutPanic(t *testing.T) {
	ran := false
	Run(func() { ran = true })
	if !ran {
		t.Fatal("the wrapped function did not run")
	}
}

// TestRecoverReportsPanicValue pins that a custom handler receives the original
// panic value, and that a clean return never invokes it.
func TestRecoverReportsPanicValue(t *testing.T) {
	t.Run("panic value is forwarded", func(t *testing.T) {
		var got any
		func() {
			defer Recover(func(err any) { got = err })
			panic("the value")
		}()
		if got != "the value" {
			t.Fatalf("handler got %v, want the panic value", got)
		}
	})

	t.Run("no panic calls no handler", func(t *testing.T) {
		called := false
		func() {
			defer Recover(func(any) { called = true })
		}()
		if called {
			t.Fatal("handler was called without a panic")
		}
	})
}

// TestGoRecoversGoroutinePanics pins that a panic on a goroutine started through
// Go cannot take the process down. The deferred send only runs during panic
// unwinding, so observing it proves the panic both happened and was contained:
// an uncontained panic in a goroutine terminates the whole process and the test
// run with it.
func TestGoRecoversGoroutinePanics(t *testing.T) {
	var ran atomic.Bool
	done := make(chan struct{})

	Go(func() {
		defer close(done)
		ran.Store(true)
		panic("background boom")
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the goroutine never completed")
	}
	if !ran.Load() {
		t.Fatal("the goroutine body did not run")
	}
}

// TestRecover_MultipleCallbacks tests that all callbacks passed to Recover are invoked in order.
func TestRecover_MultipleCallbacks(t *testing.T) {
	var results []string
	func() {
		defer Recover(
			func(err any) { results = append(results, "first:"+err.(string)) },
			func(err any) { results = append(results, "second:"+err.(string)) },
			func(err any) { results = append(results, "third:"+err.(string)) },
		)
		panic("multi")
	}()

	if len(results) != 3 {
		t.Fatalf("expected 3 callback invocations, got %d", len(results))
	}
	if results[0] != "first:multi" || results[1] != "second:multi" || results[2] != "third:multi" {
		t.Fatalf("unexpected callback results: %v", results)
	}
}

// TestLogRecovered directly executes LogRecovered to verify structured format logging without panic.
func TestLogRecovered(t *testing.T) {
	// Should not panic
	LogRecovered("test_module", "something broke")
}

// TestSafeStress_ConcurrentPanicStorm stress tests safe.Go and safe.Run under a massive
// panic storm of diverse types across 200+ goroutines.
func TestSafeStress_ConcurrentPanicStorm(t *testing.T) {
	const goroutines = 200
	var completed atomic.Int64
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// safe.Go panic storm
	for i := range goroutines {
		idx := i
		Go(func() {
			defer wg.Done()
			defer completed.Add(1)

			switch idx % 5 {
			case 0:
				panic("string panic payload")
			case 1:
				panic(errors.New("error panic payload"))
			case 2:
				panic(12345)
			case 3:
				panic(struct{ msg string }{"custom struct"})
			case 4:
				// Panic with nil in Go 1.21+ is wrapped in runtime.PanicNilError
				panic(nil)
			}
		})
	}

	// safe.Run panic storm
	for i := range goroutines {
		idx := i
		go func() {
			defer wg.Done()
			Run(func() {
				defer completed.Add(1)
				switch idx % 5 {
				case 0:
					panic(fmt.Sprintf("run panic %d", idx))
				case 1:
					panic(errors.New("run error"))
				case 2:
					panic(idx)
				case 3:
					panic([]byte("bytes panic"))
				case 4:
					panic(nil)
				}
			})
		}()
	}

	wg.Wait()

	if completed.Load() != int64(goroutines*2) {
		t.Fatalf("expected %d completed functions, got %d", goroutines*2, completed.Load())
	}
}

// TestSafeStress_NestedPanics verifies that nested safe.Go and safe.Run invocations
// contain panics at every layer without deadlocking or escaping.
func TestSafeStress_NestedPanics(t *testing.T) {
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(iterations)

	for range iterations {
		Go(func() {
			defer wg.Done()

			// Layer 1
			Run(func() {
				defer func() {
					// Layer 2
					Run(func() {
						defer func() {
							// Layer 3 inside safe.Go
							innerDone := make(chan struct{})
							Go(func() {
								defer close(innerDone)
								panic("deep inner panic")
							})
							<-innerDone
						}()
						panic("layer 2 panic")
					})
				}()
				panic("layer 1 panic")
			})
		})
	}

	wg.Wait()
}

// TestSafeStress_ConcurrentRecoverCallbacks tests safe.Recover with multiple handlers under load.
func TestSafeStress_ConcurrentRecoverCallbacks(t *testing.T) {
	const count = 100
	var wg sync.WaitGroup
	wg.Add(count)

	var totalCallbacks atomic.Int64

	for i := range count {
		go func(val int) {
			defer wg.Done()
			defer Recover(
				func(err any) {
					totalCallbacks.Add(1)
				},
				func(err any) {
					totalCallbacks.Add(1)
				},
				func(err any) {
					totalCallbacks.Add(1)
				},
			)
			panic(fmt.Sprintf("panic-val-%d", val))
		}(i)
	}

	wg.Wait()

	if totalCallbacks.Load() != int64(count*3) {
		t.Fatalf("expected %d callbacks invoked, got %d", count*3, totalCallbacks.Load())
	}
}
