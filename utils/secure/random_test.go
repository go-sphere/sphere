package secure

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestRandString pins the documented contract: the requested length, only
// alphanumeric characters, and no repeats in practice.
func TestRandString(t *testing.T) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	t.Run("length and alphabet", func(t *testing.T) {
		for _, length := range []int{1, 8, 32} {
			got := RandString(length)
			if len(got) != length {
				t.Fatalf("RandString(%d) has length %d", length, len(got))
			}
			for _, r := range got {
				if !strings.ContainsRune(chars, r) {
					t.Fatalf("RandString(%d) contains %q, want alphanumeric only", length, r)
				}
			}
		}
	})

	t.Run("non-positive length", func(t *testing.T) {
		for _, length := range []int{0, -1, -100} {
			if got := RandString(length); got != "" {
				t.Fatalf("RandString(%d) = %q, want empty", length, got)
			}
		}
	})

	t.Run("distinct draws", func(t *testing.T) {
		seen := make(map[string]struct{})
		for range 20 {
			got := RandString(24)
			if _, dup := seen[got]; dup {
				t.Fatalf("RandString produced a duplicate: %q", got)
			}
			seen[got] = struct{}{}
		}
	})
}

// TestRandStringStress tests RandString across extreme lengths and high concurrency with -race.
func TestRandStringStress(t *testing.T) {
	t.Run("boundary lengths", func(t *testing.T) {
		testCases := []struct {
			name      string
			length    int
			wantLen   int
			wantEmpty bool
		}{
			{"negative 999999", -999999, 0, true},
			{"negative 100", -100, 0, true},
			{"negative 1", -1, 0, true},
			{"zero", 0, 0, true},
			{"one", 1, 1, false},
			{"two", 2, 2, false},
			{"large 10000", 10000, 10000, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				got := RandString(tc.length)
				if tc.wantEmpty {
					if got != "" {
						t.Fatalf("RandString(%d) = %q, want empty string", tc.length, got)
					}
				} else {
					if len(got) != tc.wantLen {
						t.Fatalf("RandString(%d) len = %d, want %d", tc.length, len(got), tc.wantLen)
					}
					// Verify characters are alphanumeric
					const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
					for _, r := range got {
						if !strings.ContainsRune(chars, r) {
							t.Fatalf("RandString(%d) contains invalid rune: %q", tc.length, r)
						}
					}
				}
			})
		}
	})

	t.Run("concurrency 100 goroutines", func(t *testing.T) {
		const numGoroutines = 100
		const iterations = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		errCh := make(chan error, numGoroutines*iterations)

		for i := range numGoroutines {
			go func(workerID int) {
				defer wg.Done()
				lengths := []int{-5, 0, 1, 8, 32, 256, 10000}
				for iter := range iterations {
					l := lengths[(workerID+iter)%len(lengths)]
					s := RandString(l)
					if l <= 0 {
						if s != "" {
							errCh <- fmt.Errorf("worker %d: RandString(%d) got non-empty %q", workerID, l, s)
							return
						}
					} else {
						if len(s) != l {
							errCh <- fmt.Errorf("worker %d: RandString(%d) got len %d", workerID, l, len(s))
							return
						}
					}
				}
			}(i)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Fatal(err)
		}
	})
}
