package captcha

import (
	"testing"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

func TestRandomCode(t *testing.T) {
	t.Parallel()

	if got := RandomCode(0); got != "" {
		t.Errorf("RandomCode(0) = %q, want empty string", got)
	}

	const length = 6
	// Codes are security tokens, so a broken generator that returns a constant
	// (or an all-zero buffer) must not slip through. Distinct values across many
	// draws is a cheap smoke test for that; collisions at 10^6 are rare enough
	// that requiring most draws to differ is stable.
	seen := make(map[string]struct{}, 200)
	for range 200 {
		code := RandomCode(length)
		if len(code) != length {
			t.Fatalf("RandomCode(%d) = %q, want length %d", length, code, length)
		}
		for i := range code {
			if code[i] < '0' || code[i] > '9' {
				t.Fatalf("RandomCode produced non-digit %q in %q", code[i], code)
			}
		}
		seen[code] = struct{}{}
	}
	if len(seen) < 190 {
		t.Errorf("RandomCode produced only %d distinct values out of 200 draws", len(seen))
	}
}

type noopSender struct{}

func (noopSender) SendCode(string, string) error {
	return nil
}

func TestManagerLifecycleContract(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		return NewManager(Config{}, noopSender{})
	})
}
