package secure

import (
	"strings"
	"testing"
)

// TestRandString pins the documented contract: the requested length and only
// alphanumeric characters.
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
}
