package captcha

import (
	"testing"
	"time"

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

// TestZeroConfigDoesNotDisableVerification pins the two silent failure modes of
// a partially filled Config. A zero CodeLength used to produce an empty code,
// which Verify then accepted from anyone who could trigger a single SendCode for
// the target number. A zero CodeExpiresIn used to expire the code at the instant
// it was stored, so the message was sent and the quota spent while no
// verification could ever succeed.
func TestZeroConfigDoesNotDisableVerification(t *testing.T) {
	sender := &recordingSender{}
	m := NewManager(Config{}, sender)

	const number = "13800000000"
	if err := m.SendCode(number); err != nil {
		t.Fatalf("SendCode: %v", err)
	}

	if m.Verify(number, "") {
		t.Fatal("an empty code must never verify")
	}
	if sender.code == "" {
		t.Fatal("a zero CodeLength must fall back to the default, not send an empty code")
	}
	if len(sender.code) != DefaultCodeLength {
		t.Fatalf("expected a %d digit code, got %q", DefaultCodeLength, sender.code)
	}
	if !m.Verify(number, sender.code) {
		t.Fatal("a zero CodeExpiresIn must fall back to the default, not expire immediately")
	}
}

// TestNegativeConfigDoesNotPanic pins that a negative length coming from
// configuration cannot crash the request path; it used to panic in make with
// "len out of range".
func TestNegativeConfigDoesNotPanic(t *testing.T) {
	sender := &recordingSender{}
	m := NewManager(Config{CodeLength: -1, CodeExpiresIn: -1}, sender)

	if err := m.SendCode("13800000001"); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	if len(sender.code) != DefaultCodeLength {
		t.Fatalf("expected a %d digit code, got %q", DefaultCodeLength, sender.code)
	}
}

// TestEmptyCodeIsNeverStored pins the storage-side half of the guarantee, so no
// custom generator can install an empty credential.
func TestEmptyCodeIsNeverStored(t *testing.T) {
	s := NewVerificationSystem(VerificationConfig{})
	if err := s.SaveCode("13800000002", "", time.Minute); err == nil {
		t.Fatal("expected SaveCode to reject an empty code")
	}
	if s.Verify("13800000002", "") {
		t.Fatal("an empty code must never verify")
	}
}

type recordingSender struct {
	number string
	code   string
}

func (r *recordingSender) SendCode(number, code string) error {
	r.number = number
	r.code = code
	return nil
}
