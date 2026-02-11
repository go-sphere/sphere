package captcha

import (
	"errors"
	"math/rand/v2"
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

	MinuteTimestamps map[string]time.Time `json:"minute_timestamps"`
	DailyTimestamps  map[string]time.Time `json:"daily_timestamps"`
}

func (s *VerificationStorage) cleanExpired(number string, now time.Time) {
	if captcha, ok := s.Store[number]; ok {
		var validCaptcha []VerificationCode
		for _, capt := range captcha {
			if capt.ExpiresAt.After(now) {
				validCaptcha = append(validCaptcha, capt)
			}
		}
		s.Store[number] = validCaptcha
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
		},
	}
}

// SaveCode stores a verification code for the given number with rate limiting enforcement.
// It checks both minute and daily limits before saving the code and returns an error
// if the limits are exceeded. The code will expire after the specified duration.
func (s *VerificationSystem) SaveCode(number string, code string, expiresIn time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.store.cleanExpired(number, now)

	if count, ok := s.store.MinuteCounts[number]; ok {
		if count >= s.config.MinuteLimit {
			lastSent := s.store.MinuteTimestamps[number]
			if now.Sub(lastSent) < time.Minute {
				return errors.New("minute limit exceeded")
			} else {
				s.store.MinuteCounts[number] = 0
			}
		}
	}

	if count, ok := s.store.DailyCounts[number]; ok {
		if count >= s.config.DailyLimit {
			lastSent := s.store.DailyTimestamps[number]
			if now.Sub(lastSent) < 24*time.Hour {
				return errors.New("daily limit exceeded")
			} else {
				s.store.DailyCounts[number] = 0
			}
		}
	}

	s.store.MinuteCounts[number]++
	s.store.DailyCounts[number]++
	s.store.MinuteTimestamps[number] = now
	s.store.DailyTimestamps[number] = now

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
func (s *VerificationSystem) Verify(number, code string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	s.store.cleanExpired(number, now)
	if caps, ok := s.store.Store[number]; ok {
		for _, captcha := range caps {
			if captcha.Code == code {
				return true
			}
		}
	}
	return false
}

// CleanExpired removes all expired verification codes from storage across all numbers.
// This method should be called periodically to prevent memory leaks from accumulated expired codes.
func (s *VerificationSystem) CleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for number := range s.store.Store {
		s.store.cleanExpired(number, now)
	}
}

func (s *VerificationSystem) GetCaptchaCount(number string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.store.Store[number])
}

// RandomCode generates a random numeric verification code of the specified length.
// Each digit is randomly selected from 0-9. Returns an empty string if length is 0.
func RandomCode(length int) string {
	code := make([]byte, length)
	for i := range code {
		n := rand.IntN(10)
		code[i] = byte('0' + n)
	}
	return string(code)
}
