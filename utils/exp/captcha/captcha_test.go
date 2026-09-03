package captcha

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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

// TestCaptchaAdversarialStress tests captcha under extreme concurrency with 64+ goroutines,
// lifecycle churn, heavy send/verify contention, and lockout guarantees under -race.
func TestCaptchaAdversarialStress(t *testing.T) {
	t.Parallel()

	t.Run("heavy concurrent senders and verifiers with 64 goroutines", func(t *testing.T) {
		t.Parallel()

		sender := newConcurrentThreadSafeSender()
		mgr := NewManager(Config{
			CodeLength:    6,
			CodeExpiresIn: 300,
			RateLimit: VerificationConfig{
				MinuteLimit: 500,
				DailyLimit:  2000,
			},
		}, sender)

		ctx := t.Context()

		startErrCh := make(chan error, 1)
		go func() {
			startErrCh <- mgr.Start(ctx)
		}()

		const numWorkers = 64
		const iterations = 30
		var wg sync.WaitGroup
		wg.Add(numWorkers)

		var (
			totalSent     atomic.Int64
			totalVerified atomic.Int64
		)

		for w := range numWorkers {
			go func(wid int) {
				defer wg.Done()
				number := fmt.Sprintf("188%08d", wid%10) // 10 shared numbers to induce contention
				for range iterations {
					if err := mgr.SendCode(number); err == nil {
						totalSent.Add(1)
					}
					// Concurrently attempt to verify
					mgr.Verify(number, "000000") // likely wrong code
					mgr.Verify(number, "")       // empty code must always fail
				}
			}(w)
		}

		wg.Wait()

		// Concurrently invoke CleanExpired
		var cleanWg sync.WaitGroup
		cleanWg.Add(10)
		for range 10 {
			go func() {
				defer cleanWg.Done()
				mgr.verification.CleanExpired()
			}()
		}
		cleanWg.Wait()

		// Stop manager gracefully
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()

		if err := mgr.Stop(stopCtx); err != nil {
			t.Fatalf("Stop failed: %v", err)
		}

		select {
		case err := <-startErrCh:
			if err != nil {
				t.Fatalf("Start returned error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Start routine did not terminate after Stop")
		}

		t.Logf("Heavy concurrency test completed: sent=%d, verified=%d", totalSent.Load(), totalVerified.Load())
	})

	t.Run("rapid start and stop cycles", func(t *testing.T) {
		t.Parallel()

		for i := range 30 {
			sender := newConcurrentThreadSafeSender()
			mgr := NewManager(Config{}, sender)

			startCtx, startCancel := context.WithCancel(context.Background())
			doneCh := make(chan error, 1)
			go func() {
				doneCh <- mgr.Start(startCtx)
			}()

			// Send some codes
			_ = mgr.SendCode(fmt.Sprintf("150%08d", i))

			// Randomly cancel context OR call Stop
			if i%2 == 0 {
				startCancel()
			} else {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
				_ = mgr.Stop(stopCtx)
				stopCancel()
				startCancel()
			}

			select {
			case <-doneCh:
			case <-time.After(2 * time.Second):
				t.Fatalf("iteration %d: Start did not terminate promptly", i)
			}
		}
	})

	t.Run("lockout preservation under heavy concurrent brute-force", func(t *testing.T) {
		t.Parallel()

		sys := NewVerificationSystem(VerificationConfig{
			MinuteLimit: 100,
			DailyLimit:  1000,
		})

		const number = "19900000001"
		const correctCode = "123456"

		if err := sys.SaveCode(number, correctCode, 10*time.Minute); err != nil {
			t.Fatalf("SaveCode failed: %v", err)
		}

		const numAttackers = 60
		var attackWg sync.WaitGroup
		attackWg.Add(numAttackers)

		for i := range numAttackers {
			go func(attackerID int) {
				defer attackWg.Done()
				wrong := fmt.Sprintf("%06d", (attackerID+1)*11111%999999)
				if wrong == correctCode {
					wrong = "000000"
				}
				sys.Verify(number, wrong)
			}(i)
		}

		attackWg.Wait()

		// Verify lockout state
		sys.mu.RLock()
		lockedUntil, isLocked := sys.store.LockedUntil[number]
		codes := sys.store.Store[number]
		sys.mu.RUnlock()

		if !isLocked || time.Now().After(lockedUntil) {
			t.Fatalf("expected number %s to be locked out, got isLocked=%v, lockedUntil=%v", number, isLocked, lockedUntil)
		}

		// The legitimate code MUST be preserved
		if len(codes) == 0 || codes[0].Code != correctCode {
			t.Fatalf("legitimate code was lost during brute-force: %+v", codes)
		}

		// Correct code must fail while locked
		if sys.Verify(number, correctCode) {
			t.Fatal("Verify accepted code during active lockout")
		}

		// Fast-forward lockout
		sys.mu.Lock()
		sys.store.LockedUntil[number] = time.Now().Add(-1 * time.Second)
		sys.mu.Unlock()

		// Once unlocked, legitimate code must succeed
		if !sys.Verify(number, correctCode) {
			t.Fatal("Verify failed for legitimate code after lockout elapsed")
		}

		// Subsequent verification must fail (consumed)
		if sys.Verify(number, correctCode) {
			t.Fatal("Verify succeeded second time; replay protection violated")
		}
	})
}

// concurrentThreadSafeSender records sends in a thread-safe manner
type concurrentThreadSafeSender struct {
	mu     sync.Mutex
	counts map[string]int
}

func newConcurrentThreadSafeSender() *concurrentThreadSafeSender {
	return &concurrentThreadSafeSender{
		counts: make(map[string]int),
	}
}

func (s *concurrentThreadSafeSender) SendCode(number, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[number]++
	return nil
}

// TestManagerTickerCleanupLifecycleStress verifies Manager Start/Stop lifecycle and cleanup under concurrency.
func TestManagerTickerCleanupLifecycleStress(t *testing.T) {
	sender := newConcurrentThreadSafeSender()
	m := NewManager(Config{
		CodeLength:    6,
		CodeExpiresIn: 1, // 1 second expiration
		RateLimit: VerificationConfig{
			MinuteLimit: 1000,
			DailyLimit:  5000,
		},
	}, sender)

	ctx := t.Context()

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.Start(ctx)
	}()

	// Concurrently send codes for 50 distinct numbers
	const numSenders = 50
	var wg sync.WaitGroup
	wg.Add(numSenders)

	for i := range numSenders {
		go func(id int) {
			defer wg.Done()
			num := fmt.Sprintf("139%08d", id)
			if err := m.SendCode(num); err != nil {
				t.Errorf("SendCode failed for %s: %v", num, err)
			}
		}(i)
	}
	wg.Wait()

	// Direct call to CleanExpired concurrently with Manager running
	for range 5 {
		m.verification.CleanExpired()
		time.Sleep(10 * time.Millisecond)
	}

	// Graceful shutdown via Stop
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	if err := m.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Multiple calls to Stop must be safe (sync.Once)
	if err := m.Stop(stopCtx); err != nil {
		t.Fatalf("Second Stop failed: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start exited with error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start routine did not terminate after Stop")
	}
}
