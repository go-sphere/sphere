package captcha

import (
	"testing"
	"time"
)

func newTestSystem() *VerificationSystem {
	return NewVerificationSystem(VerificationConfig{MinuteLimit: 100, DailyLimit: 1000})
}

// TestVerifyDoesNotCountAttemptsForUnknownNumber pins that probing an arbitrary
// number cannot grow the failure map: there is no outstanding code to protect.
func TestVerifyDoesNotCountAttemptsForUnknownNumber(t *testing.T) {
	s := newTestSystem()
	for range 10 {
		if s.Verify("never-sent", "000000") {
			t.Fatal("Verify should reject a number with no outstanding code")
		}
	}
	if got := len(s.store.FailedAttempts); got != 0 {
		t.Errorf("FailedAttempts grew to %d entries for a number that was never sent a code", got)
	}
}

// TestVerifyLockoutBurnsOutstandingCodes covers the brute-force response and the
// permanent-lockout trap it used to create: exhausting the attempt budget must
// invalidate the outstanding codes without leaving a counter behind that a number
// with an exhausted send quota could never clear.
func TestVerifyLockoutBurnsOutstandingCodes(t *testing.T) {
	const number = "13800000000"
	s := newTestSystem()
	if err := s.SaveCode(number, "123456", time.Minute); err != nil {
		t.Fatalf("SaveCode: %v", err)
	}

	for i := range DefaultMaxAttempts {
		if s.Verify(number, "999999") {
			t.Fatalf("Verify accepted a wrong code on attempt %d", i+1)
		}
	}

	// The outstanding code is burned: even the correct value no longer verifies.
	if s.Verify(number, "123456") {
		t.Error("the outstanding code should have been invalidated by the failed attempts")
	}
	// And no bookkeeping is left behind for it.
	if _, ok := s.store.FailedAttempts[number]; ok {
		t.Error("failure counter outlived the codes it protected")
	}
	if _, ok := s.store.Store[number]; ok {
		t.Error("burned codes should have been removed from storage")
	}

	// Re-issuing must restore a working verification path.
	if err := s.SaveCode(number, "654321", time.Minute); err != nil {
		t.Fatalf("SaveCode after lockout: %v", err)
	}
	if !s.Verify(number, "654321") {
		t.Error("a freshly issued code must verify after a lockout")
	}
}

// TestVerifyConsumesMatchedCode pins the one-time-use property.
func TestVerifyConsumesMatchedCode(t *testing.T) {
	const number = "13700000000"
	s := newTestSystem()
	if err := s.SaveCode(number, "123456", time.Minute); err != nil {
		t.Fatalf("SaveCode: %v", err)
	}
	if !s.Verify(number, "123456") {
		t.Fatal("first verification should succeed")
	}
	if s.Verify(number, "123456") {
		t.Error("a matched code must not be replayable")
	}
	if _, ok := s.store.FailedAttempts[number]; ok {
		t.Error("consuming the last code should not leave a failure counter behind")
	}
}

// TestCleanExpiredReleasesIdleNumbers pins that the storage maps do not retain one
// entry per number seen forever.
func TestCleanExpiredReleasesIdleNumbers(t *testing.T) {
	s := newTestSystem()
	if err := s.SaveCode("13900000000", "123456", 10*time.Millisecond); err != nil {
		t.Fatalf("SaveCode: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	// The daily window has not elapsed, so rate-limit state must survive while
	// the expired code itself is dropped.
	s.CleanExpired()
	if _, ok := s.store.Store["13900000000"]; ok {
		t.Error("expired code should have been dropped")
	}
	if _, ok := s.store.DailyCounts["13900000000"]; !ok {
		t.Error("daily rate-limit state must survive within its window")
	}

	// Once the daily window has passed the number is fully idle and released.
	s.store.DailyTimestamps["13900000000"] = time.Now().Add(-25 * time.Hour)
	s.CleanExpired()
	for name, size := range map[string]int{
		"Store":            len(s.store.Store),
		"MinuteCounts":     len(s.store.MinuteCounts),
		"DailyCounts":      len(s.store.DailyCounts),
		"MinuteTimestamps": len(s.store.MinuteTimestamps),
		"DailyTimestamps":  len(s.store.DailyTimestamps),
		"FailedAttempts":   len(s.store.FailedAttempts),
	} {
		if size != 0 {
			t.Errorf("%s retained %d entries for an idle number", name, size)
		}
	}
}
