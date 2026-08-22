package secure

import (
	"strings"
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

	t.Run("zero length", func(t *testing.T) {
		if got := RandString(0); got != "" {
			t.Fatalf("RandString(0) = %q, want empty", got)
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
