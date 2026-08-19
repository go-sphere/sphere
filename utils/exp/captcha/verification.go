package captcha

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sync"
	"time"
)

const (
	// DefaultMinuteLimit defines the default maximum number of verification codes that can be sent per minute.
	DefaultMinuteLimit = 1
	// DefaultDailyLimit defines the default maximum number of verification codes that can be sent per day.
	DefaultDailyLimit = 100
	// DefaultStoreSize defines the default initial capacity for the verification storage maps.
	DefaultStoreSize = 100
	// DefaultMaxAttempts defines the maximum number of failed verification attempts allowed
	// for a number before further attempts are frozen to prevent brute forcing.
	DefaultMaxAttempts = 5
	// DefaultLockoutWindow is how long a number stops accepting verification
	// attempts once DefaultMaxAttempts failures accumulate.
	DefaultLockoutWindow = 15 * time.Minute
)

// VerificationConfig holds the rate limiting configuration for verification code generation.
type VerificationConfig struct {
	MinuteLimit int `json:"minute_limit"`
	DailyLimit  int `json:"daily_limit"`
}

// VerificationCode represents a verification code with its expiration time.
type VerificationCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type VerificationStorage struct {
	Store map[string][]VerificationCode `json:"store"`

	MinuteCounts map[string]int `json:"minute_counts"`
	DailyCounts  map[string]int `json:"daily_counts"`

	// MinuteTimestamps and DailyTimestamps record the start of each number's
	// current rate-limit window, not the time of its last send. Anchoring them to
	// the last send would stop the window from ever rolling for a number that
	// keeps using it.
	MinuteTimestamps map[string]time.Time `json:"minute_timestamps"`
	DailyTimestamps  map[string]time.Time `json:"daily_timestamps"`

	FailedAttempts map[string]int `json:"failed_attempts"`

	// LockedUntil freezes verification attempts for a number after too many
	// failures. Freezing replaces invalidating the outstanding codes, which let
	// anyone who knew a phone number destroy the credential its owner had just
	// been sent.
	LockedUntil map[string]time.Time `json:"locked_until"`
}

func (s *VerificationStorage) cleanExpired(number string, now time.Time) {
	captcha, ok := s.Store[number]
	if !ok {
		return
	}
	var validCaptcha []VerificationCode
	for _, capt := range captcha {
		if capt.ExpiresAt.After(now) {
			validCaptcha = append(validCaptcha, capt)
		}
	}
	if len(validCaptcha) == 0 {
		// Drop the key entirely instead of leaving an empty slice behind, so the
		// map does not retain one entry per number ever seen.
		delete(s.Store, number)
		// The failure counter and freeze exist to protect outstanding codes from
		// brute forcing. With no code left there is nothing to guess, so keeping
		// them would only leak memory.
		delete(s.FailedAttempts, number)
		delete(s.LockedUntil, number)
		return
	}
	s.Store[number] = validCaptcha
}

// forgetIdle drops all per-number bookkeeping once the number's daily rate-limit
// window has elapsed and it has no outstanding codes. SaveCode already resets a
// counter whose window has passed, so removing the entry is equivalent to
// keeping it and is what bounds the storage maps over time.
func (s *VerificationStorage) forgetIdle(number string, now time.Time) {
	if _, ok := s.Store[number]; ok {
		return
	}
	// A failure count or freeze without a code to protect is meaningless. Verify
	// maintains that invariant, but storage restored from JSON may not, so
	// enforce it here.
	delete(s.FailedAttempts, number)
	delete(s.LockedUntil, number)
	if last, ok := s.DailyTimestamps[number]; ok && now.Sub(last) < 24*time.Hour {
		return
	}
	delete(s.MinuteCounts, number)
	delete(s.DailyCounts, number)
	delete(s.MinuteTimestamps, number)
	delete(s.DailyTimestamps, number)
	delete(s.FailedAttempts, number)
}

// rollWindow resets a rate-limit counter whose window has elapsed and anchors a
// new window at now. It runs before the limit is checked, so the window rolls on
// its own schedule. Previously a counter was only reset once it had already hit
// its limit, which made both windows cumulative rather than rolling: a number
// used a couple of times a day kept accumulating across days until it was
// refused for exceeding a "daily" limit it never reached on any single day, and
// could only recover by going completely silent for 24 hours.
func (s *VerificationStorage) rollWindow(number string, now time.Time, window time.Duration, counts map[string]int, starts map[string]time.Time) {
	start, ok := starts[number]
	if !ok || now.Sub(start) >= window {
		counts[number] = 0
		starts[number] = now
	}
}

// VerificationSystem provides thread-safe verification code management with rate limiting.
// It handles code storage, expiration cleanup, and enforces sending limits per phone number.
type VerificationSystem struct {
	mu     sync.RWMutex
	config VerificationConfig
	store  *VerificationStorage
}

// NewVerificationSystem creates a new verification system with the provided configuration.
// If configuration fields are zero, it uses default values for rate limiting and storage capacity.
func NewVerificationSystem(conf VerificationConfig) *VerificationSystem {
	if conf.MinuteLimit == 0 {
		conf.MinuteLimit = DefaultMinuteLimit
	}
	if conf.DailyLimit == 0 {
		conf.DailyLimit = DefaultDailyLimit
	}
	return &VerificationSystem{
		config: conf,
		store: &VerificationStorage{
			Store:            make(map[string][]VerificationCode, DefaultStoreSize),
			MinuteCounts:     make(map[string]int, DefaultStoreSize),
			DailyCounts:      make(map[string]int, DefaultStoreSize),
			MinuteTimestamps: make(map[string]time.Time, DefaultStoreSize),
			DailyTimestamps:  make(map[string]time.Time, DefaultStoreSize),
			FailedAttempts:   make(map[string]int, DefaultStoreSize),
			LockedUntil:      make(map[string]time.Time, DefaultStoreSize),
		},
	}
}

// SaveCode stores a verification code for the given number with rate limiting enforcement.
// It checks both minute and daily limits before saving the code and returns an error
// if the limits are exceeded. The code will expire after the specified duration.
func (s *VerificationSystem) SaveCode(number string, code string, expiresIn time.Duration) error {
	// An empty code would be matched by an empty submission, turning verification
	// into a no-op for this number. Refuse to store one so a misconfigured
	// generator fails loudly instead of silently disabling the check.
	if code == "" {
		return errors.New("captcha: refusing to store an empty code")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.store.cleanExpired(number, now)

	s.store.rollWindow(number, now, time.Minute, s.store.MinuteCounts, s.store.MinuteTimestamps)
	s.store.rollWindow(number, now, 24*time.Hour, s.store.DailyCounts, s.store.DailyTimestamps)

	if s.store.MinuteCounts[number] >= s.config.MinuteLimit {
		return errors.New("minute limit exceeded")
	}
	if s.store.DailyCounts[number] >= s.config.DailyLimit {
		return errors.New("daily limit exceeded")
	}

	s.store.MinuteCounts[number]++
	s.store.DailyCounts[number]++
	s.store.FailedAttempts[number] = 0

	newCaptcha := VerificationCode{
		Code:      code,
		ExpiresAt: now.Add(expiresIn),
	}
	s.store.Store[number] = append(s.store.Store[number], newCaptcha)
	return nil
}

// Verify checks if the provided verification code is valid for the given number.
// It returns true if a matching, non-expired code is found, false otherwise.
// Expired codes are automatically cleaned up during verification.
//
// A matched code is consumed immediately (one-time use), preventing replay after a
// successful verification. Failed attempts are throttled: once DefaultMaxAttempts
// failures accumulate for a number, all of its outstanding codes are invalidated to
// prevent brute forcing. A newly issued code (via SaveCode) resets the failure counter.
//
// Attempts against a number with no outstanding code are rejected without being
// counted: there is nothing to brute force, and counting them would let any caller
// grow the failure map with arbitrary numbers.
func (s *VerificationSystem) Verify(number, code string) bool {
	// Never let an empty submission match. SaveCode already refuses to store an
	// empty code; this is the second half of the same guarantee, so no storage
	// path can make an empty string a valid credential.
	if code == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.store.cleanExpired(number, now)

	if until, ok := s.store.LockedUntil[number]; ok {
		if now.Before(until) {
			// Frozen: reject without comparing, and without touching the codes.
			return false
		}
		delete(s.store.LockedUntil, number)
		delete(s.store.FailedAttempts, number)
	}

	caps, ok := s.store.Store[number]
	if !ok || len(caps) == 0 {
		return false
	}

	for i, captcha := range caps {
		if captcha.Code == code {
			// Consume the matched code so it cannot be replayed.
			remaining := append(caps[:i], caps[i+1:]...)
			if len(remaining) == 0 {
				delete(s.store.Store, number)
				delete(s.store.FailedAttempts, number)
			} else {
				s.store.Store[number] = remaining
				s.store.FailedAttempts[number] = 0
			}
			return true
		}
	}

	s.store.FailedAttempts[number]++
	if s.store.FailedAttempts[number] >= DefaultMaxAttempts {
		// Too many failures: freeze attempts for this number, but keep its
		// outstanding codes. Invalidating them instead spent the attacker's
		// guesses on destroying the victim's credential — anyone who knew a phone
		// number could wipe the code its owner had just received, and with the
		// send limit blocking an immediate resend, keep them locked out
		// indefinitely for free. Freezing bounds the damage to the lockout window
		// and leaves the legitimate code usable once it elapses.
		s.store.LockedUntil[number] = now.Add(DefaultLockoutWindow)
		s.store.FailedAttempts[number] = 0
	}
	return false
}

// CleanExpired removes all expired verification codes from storage across all numbers,
// and drops the rate-limit and failure bookkeeping for numbers that have gone idle.
// This method should be called periodically to prevent memory leaks from accumulated
// expired codes; captcha.Manager does so once a minute.
func (s *VerificationSystem) CleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()

	numbers := make([]string, 0, len(s.store.Store))
	for number := range s.store.Store {
		numbers = append(numbers, number)
	}
	for _, number := range numbers {
		s.store.cleanExpired(number, now)
	}

	// Numbers whose codes are already gone still hold counters, so sweep the
	// rate-limit maps too rather than only the ones that had codes this round.
	numbers = numbers[:0]
	for number := range s.store.DailyTimestamps {
		numbers = append(numbers, number)
	}
	for _, number := range numbers {
		s.store.forgetIdle(number, now)
	}
}

func (s *VerificationSystem) GetCaptchaCount(number string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.store.Store[number])
}

// RandomCode generates a random numeric verification code of the specified length.
// Each digit is drawn from crypto/rand so the issued code is not predictable from
// previously observed codes; a verification code is a security token and must not
// come from a non-cryptographic generator. Returns an empty string if length is 0.
// It panics if the system entropy source fails, since that is an unrecoverable
// condition and returning a guessable code instead would be worse.
func RandomCode(length int) string {
	// A negative length would panic in make; report it as "no code" and let the
	// empty-code guards in SaveCode/Verify reject it.
	if length <= 0 {
		return ""
	}
	code := make([]byte, length)
	digits := big.NewInt(10)
	for i := range code {
		n, err := rand.Int(rand.Reader, digits)
		if err != nil {
			panic("captcha: crypto/rand failed: " + err.Error())
		}
		code[i] = byte('0' + n.Int64())
	}
	return string(code)
}
