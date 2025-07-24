package captcha

import (
	"errors"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	DefaultMinuteLimit = 1
	DefaultDailyLimit  = 100
	DefaultStoreSize   = 100
)

type VerificationConfig struct {
	MinuteLimit int `json:"minute_limit"`
	DailyLimit  int `json:"daily_limit"`
}

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

type VerificationSystem struct {
	mu     sync.RWMutex
	config *VerificationConfig
	store  *VerificationStorage
}

func NewVerificationSystem(config *VerificationConfig) *VerificationSystem {
	if config == nil {
		config = &VerificationConfig{
			MinuteLimit: DefaultMinuteLimit,
			DailyLimit:  DefaultDailyLimit,
		}
	}
	return &VerificationSystem{
		config: config,
		store: &VerificationStorage{
			Store:            make(map[string][]VerificationCode, DefaultStoreSize),
			MinuteCounts:     make(map[string]int, DefaultStoreSize),
			DailyCounts:      make(map[string]int, DefaultStoreSize),
			MinuteTimestamps: make(map[string]time.Time, DefaultStoreSize),
			DailyTimestamps:  make(map[string]time.Time, DefaultStoreSize),
		},
	}
}

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

func RandomCode(length int) string {
	code := make([]byte, length)
	for i := range code {
		n := rand.IntN(10)
		code[i] = byte('0' + n)
	}
	return string(code)
}
