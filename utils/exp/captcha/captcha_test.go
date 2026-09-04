package captcha

import (
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

func TestRandomCode(t *testing.T) {
	t.Parallel()

	for _, length := range []int{-1, 0, 1, 6, 32} {
		code := RandomCode(length)
		wantLength := max(length, 0)
		if len(code) != wantLength {
			t.Fatalf("RandomCode(%d) = %q, want length %d", length, code, wantLength)
		}
		for i := range code {
			if code[i] < '0' || code[i] > '9' {
				t.Fatalf("RandomCode produced non-digit %q in %q", code[i], code)
			}
		}
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

	if sender.getNumber() != number {
		t.Fatalf("expected recorded number %s, got %s", number, sender.getNumber())
	}
	if m.Verify(number, "") {
		t.Fatal("an empty code must never verify")
	}
	if sender.getCode() == "" {
		t.Fatal("a zero CodeLength must fall back to the default, not send an empty code")
	}
	if len(sender.getCode()) != DefaultCodeLength {
		t.Fatalf("expected a %d digit code, got %q", DefaultCodeLength, sender.getCode())
	}
	if !m.Verify(number, sender.getCode()) {
		t.Fatal("a zero CodeExpiresIn must fall back to the default, not expire immediately")
	}
}

// TestNegativeConfigDoesNotPanic pins that a negative length coming from
// configuration cannot crash the request path; it used to panic in make with
// "len out of range".
func TestNegativeConfigDoesNotPanic(t *testing.T) {
	sender := &recordingSender{}
	m := NewManager(Config{CodeLength: -1, CodeExpiresIn: -1}, sender)

	const number = "13800000001"
	if err := m.SendCode(number); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	if sender.getNumber() != number {
		t.Fatalf("expected recorded number %s, got %s", number, sender.getNumber())
	}
	if len(sender.getCode()) != DefaultCodeLength {
		t.Fatalf("expected a %d digit code, got %q", DefaultCodeLength, sender.getCode())
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
	mu     sync.Mutex
	number string
	code   string
}

func (r *recordingSender) SendCode(number, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.number = number
	r.code = code
	return nil
}

func (r *recordingSender) getCode() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.code
}

func (r *recordingSender) getNumber() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.number
}
