// Package captcha provides a complete verification code management system with rate limiting.
// It supports configurable code generation, delivery through pluggable senders,
// automatic expiration handling, and protection against abuse through minute/daily limits.
package captcha

import (
	"context"
	"time"
)

// Sender defines the interface for sending verification codes to recipients.
// Implementations should handle the actual delivery mechanism (SMS, email, etc.).
type Sender interface {
	SendCode(number string, code string) error
}

// Config holds the configuration parameters for the verification code system.
type Config struct {
	CodeLength    int                 `json:"code_length"`     // Length of generated verification codes
	CodeExpiresIn int                 `json:"code_expires_in"` // Code expiration time in seconds
	RateLimit     *VerificationConfig `json:"rate_limit"`      // Rate limiting configuration
}

// Manager provides verification code generation, sending, and validation capabilities.
// It combines code generation, delivery, and rate limiting in a single component.
type Manager struct {
	done         chan struct{}       // Channel for graceful shutdown signaling
	config       *Config             // Manager configuration
	sender       Sender              // Code delivery implementation
	verification *VerificationSystem // Rate limiting and validation system
}

// NewManager creates a new verification code manager with the provided configuration and sender.
// It initializes the verification system with rate limiting capabilities.
func NewManager(config *Config, sender Sender) *Manager {
	return &Manager{
		done:         make(chan struct{}),
		config:       config,
		sender:       sender,
		verification: NewVerificationSystem(config.RateLimit),
	}
}

// SendCode generates and sends a verification code to the specified number.
// It creates a random code, saves it with expiration, and delivers it using the configured sender.
// Returns an error if code generation, storage, or delivery fails.
func (m *Manager) SendCode(number string) error {
	code := RandomCode(m.config.CodeLength)
	err := m.verification.SaveCode(number, code, time.Duration(m.config.CodeExpiresIn)*time.Second)
	if err != nil {
		return err
	}
	err = m.sender.SendCode(number, code)
	if err != nil {
		return err
	}
	return nil
}

// Verify validates a verification code for the given number.
// Returns true if the code is valid and not expired, false otherwise.
func (m *Manager) Verify(number, code string) bool {
	return m.verification.Verify(number, code)
}

// Identifier returns the task identifier for the captcha manager.
// This implements the Task interface for lifecycle management integration.
func (m *Manager) Identifier() string {
	return "captcha"
}

// Start begins the captcha manager's background cleanup routine.
// It runs a ticker that periodically cleans expired verification codes to prevent memory leaks.
// This implements the Task interface for lifecycle management integration.
func (m *Manager) Start(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.done:
			return nil
		case <-ticker.C:
			m.verification.CleanExpired()
		}
	}
}

// Stop gracefully shuts down the captcha manager by closing the done channel.
// This signals the cleanup routine to exit and implements the Task interface.
func (m *Manager) Stop(ctx context.Context) error {
	close(m.done)
	return nil
}
