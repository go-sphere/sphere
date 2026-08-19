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

// TestVerifyLockoutPreservesOutstandingCodes covers the brute-force response.
//
// Exhausting the attempt budget must freeze further attempts, not destroy the
// code the number's owner is holding. Invalidating it spent the attacker's
// guesses on the victim's credential: knowing a phone number was enough to wipe
// the code its owner had just received, and with the send limit blocking an
// immediate resend, to keep them locked out indefinitely at no cost.
func TestVerifyLockoutPreservesOutstandingCodes(t *testing.T) {
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

	// Frozen: even the correct value is refused while the window is open.
	if s.Verify(number, "123456") {
		t.Error("Verify must refuse while the number is frozen")
	}
	// But the code itself survives the freeze.
	if _, ok := s.store.Store[number]; !ok {
		t.Fatal("the outstanding code must not be destroyed by failed attempts")
	}
	if _, ok := s.store.LockedUntil[number]; !ok {
		t.Fatal("expected the number to be frozen")
	}

	// Once the window elapses, the original code still works.
	s.store.LockedUntil[number] = time.Now().Add(-time.Second)
	if !s.Verify(number, "123456") {
		t.Error("the outstanding code must verify once the freeze elapses")
	}
	if _, ok := s.store.LockedUntil[number]; ok {
		t.Error("an elapsed freeze must be cleared")
	}
}

// TestVerifyLockoutClearedWithTheCodesItProtects pins that no bookkeeping
// outlives the codes: once the outstanding codes expire there is nothing left to
// brute force, so the freeze and counter must go with them rather than leaving a
// number locked out with no way to clear it.
func TestVerifyLockoutClearedWithTheCodesItProtects(t *testing.T) {
	const number = "13800000001"
	s := newTestSystem()
	if err := s.SaveCode(number, "123456", 10*time.Millisecond); err != nil {
		t.Fatalf("SaveCode: %v", err)
	}
	for range DefaultMaxAttempts {
		s.Verify(number, "999999")
	}
	if _, ok := s.store.LockedUntil[number]; !ok {
		t.Fatal("expected the number to be frozen")
	}

	time.Sleep(20 * time.Millisecond)
	s.CleanExpired()

	if _, ok := s.store.LockedUntil[number]; ok {
		t.Error("freeze outlived the codes it protected")
	}
	if _, ok := s.store.FailedAttempts[number]; ok {
		t.Error("failure counter outlived the codes it protected")
	}
}

// TestRateLimitWindowRolls pins that the send limits are rolling windows.
// Resetting a counter only after it had already hit its limit made them
// cumulative: a number used a couple of times a day accumulated across days
// until it was refused for a "daily" limit it never reached on any single day,
// and could only recover by going completely silent for 24 hours.
func TestRateLimitWindowRolls(t *testing.T) {
	const number = "13800000002"
	s := NewVerificationSystem(VerificationConfig{MinuteLimit: 1, DailyLimit: 3})

	for range 3 {
		if err := s.SaveCode(number, "123456", time.Minute); err != nil {
			t.Fatalf("SaveCode within the daily limit: %v", err)
		}
		// Roll the minute window forward without waiting for it.
		s.store.MinuteTimestamps[number] = time.Now().Add(-2 * time.Minute)
	}

	if err := s.SaveCode(number, "123456", time.Minute); err == nil {
		t.Fatal("expected the daily limit to be enforced")
	}

	// A day later the daily window rolls and sending works again, even though
	// the limit was never reached inside any single window.
	s.store.DailyTimestamps[number] = time.Now().Add(-25 * time.Hour)
	if err := s.SaveCode(number, "123456", time.Minute); err != nil {
		t.Fatalf("daily window must roll: %v", err)
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
