package captcha

import (
	"fmt"
	"sync"
	"sync/atomic"
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

// TestRandomCodeStress tests RandomCode across negative, 0, 1, 10000 lengths and concurrency under -race.
func TestRandomCodeStress(t *testing.T) {
	t.Run("boundary lengths", func(t *testing.T) {
		lengths := []struct {
			len       int
			wantEmpty bool
		}{
			{-9999, true},
			{-1, true},
			{0, true},
			{1, false},
			{6, false},
			{10000, false},
		}

		for _, tc := range lengths {
			code := RandomCode(tc.len)
			if tc.wantEmpty {
				if code != "" {
					t.Fatalf("RandomCode(%d) = %q, want empty", tc.len, code)
				}
			} else {
				if len(code) != tc.len {
					t.Fatalf("RandomCode(%d) len = %d, want %d", tc.len, len(code), tc.len)
				}
				for _, b := range []byte(code) {
					if b < '0' || b > '9' {
						t.Fatalf("RandomCode(%d) non-digit character %q", tc.len, string(b))
					}
				}
			}
		}
	})

	t.Run("concurrency 100 goroutines", func(t *testing.T) {
		const numGoroutines = 100
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := range numGoroutines {
			go func(workerID int) {
				defer wg.Done()
				lens := []int{-1, 0, 1, 6, 12, 1000}
				for iter := range 50 {
					l := lens[(workerID+iter)%len(lens)]
					c := RandomCode(l)
					if l <= 0 && c != "" {
						t.Errorf("RandomCode(%d) non-empty", l)
					}
					if l > 0 && len(c) != l {
						t.Errorf("RandomCode(%d) unexpected len %d", l, len(c))
					}
				}
			}(i)
		}
		wg.Wait()
	})
}

// TestVerificationStorageBruteForceLockoutStress tests concurrent brute force attempts,
// code preservation during lockout, and one-time consumption.
func TestVerificationStorageBruteForceLockoutStress(t *testing.T) {
	t.Run("concurrent brute force triggers lockout but preserves valid code", func(t *testing.T) {
		s := NewVerificationSystem(VerificationConfig{MinuteLimit: 100, DailyLimit: 1000})
		const number = "13800009999"
		const validCode = "888888"

		if err := s.SaveCode(number, validCode, 10*time.Minute); err != nil {
			t.Fatalf("SaveCode failed: %v", err)
		}

		// Hammer with 50 concurrent wrong verification attempts
		const workers = 50
		var wg sync.WaitGroup
		wg.Add(workers)

		for i := range workers {
			go func(id int) {
				defer wg.Done()
				wrongCode := fmt.Sprintf("%06d", id)
				if wrongCode == validCode {
					wrongCode = "999999"
				}
				s.Verify(number, wrongCode)
			}(i)
		}
		wg.Wait()

		// Verify lockout state
		s.mu.RLock()
		lockedUntil, isLocked := s.store.LockedUntil[number]
		codes, hasCodes := s.store.Store[number]
		s.mu.RUnlock()

		if !isLocked || time.Now().After(lockedUntil) {
			t.Fatalf("expected number %s to be locked out, got locked: %v, until: %v", number, isLocked, lockedUntil)
		}

		if !hasCodes || len(codes) == 0 || codes[0].Code != validCode {
			t.Fatalf("valid code was destroyed by brute force! store: %+v", codes)
		}

		// While locked, even the correct code MUST NOT verify
		if s.Verify(number, validCode) {
			t.Fatal("Verify accepted valid code during active lockout window")
		}

		// Advance time past lockout window
		s.mu.Lock()
		s.store.LockedUntil[number] = time.Now().Add(-1 * time.Second)
		s.mu.Unlock()

		// Now the valid code MUST verify and consume cleanly
		if !s.Verify(number, validCode) {
			t.Fatal("Verify failed for valid code after lockout expired")
		}

		// Verify one-time destruction after success
		if s.Verify(number, validCode) {
			t.Fatal("Verify succeeded second time; one-time consumption violated!")
		}

		s.mu.RLock()
		_, hasCodesAfter := s.store.Store[number]
		_, hasLockedAfter := s.store.LockedUntil[number]
		_, hasFailedAfter := s.store.FailedAttempts[number]
		s.mu.RUnlock()

		if hasCodesAfter {
			t.Fatal("code not removed from store after successful verification")
		}
		if hasLockedAfter {
			t.Fatal("lockout state not cleared after consuming last code")
		}
		if hasFailedAfter {
			t.Fatal("failed attempts counter not cleared after consuming last code")
		}
	})

	t.Run("concurrent verification of the same code allows exactly one winner", func(t *testing.T) {
		s := NewVerificationSystem(VerificationConfig{MinuteLimit: 100, DailyLimit: 1000})
		const number = "13800008888"
		const validCode = "777777"

		if err := s.SaveCode(number, validCode, 5*time.Minute); err != nil {
			t.Fatalf("SaveCode failed: %v", err)
		}

		const numGoroutines = 50
		var successCount atomic.Int64
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for range numGoroutines {
			go func() {
				defer wg.Done()
				if s.Verify(number, validCode) {
					successCount.Add(1)
				}
			}()
		}
		wg.Wait()

		if got := successCount.Load(); got != 1 {
			t.Fatalf("expected exactly 1 successful verification, got %d", got)
		}
	})
}
