package captcha

import (
	"context"
	"sync"
	"time"
)

// Sender defines the interface for sending verification codes to recipients.
// Implementations should handle the actual delivery mechanism (SMS, email, etc.).
// Manager calls SendCode synchronously from Manager.SendCode, possibly from
// several goroutines at once; a returned error is passed back to the caller.
type Sender interface {
	SendCode(number string, code string) error
}

// Config holds the configuration parameters for the verification code system.
// Non-positive CodeLength and CodeExpiresIn take their defaults.
type Config struct {
	CodeLength    int                `json:"code_length"`     // Number of digits in generated codes (default 6)
	CodeExpiresIn int                `json:"code_expires_in"` // Code expiration time in seconds (default 300)
	RateLimit     VerificationConfig `json:"rate_limit"`      // Per-number send limits
}

// Defaults applied by NewManager to a Config field left at its zero value.
// Without them a partially filled Config produces a system that looks healthy
// but cannot work: a zero CodeLength yields an empty code, which Verify then
// accepts from anyone, and a zero CodeExpiresIn expires every code at the
// instant it is stored, so the SMS is sent and the quota spent while no
// verification can ever succeed.
const (
	DefaultCodeLength    = 6
	DefaultCodeExpiresIn = 300 // seconds
)

// Manager provides verification code generation, sending, and validation capabilities.
// It combines code generation, delivery, and rate limiting in a single component.
// Create it with NewManager. It implements task.Task (Identifier, Start, Stop)
// and is safe for concurrent use.
type Manager struct {
	done         chan struct{}       // Channel for graceful shutdown signaling
	stopOnce     sync.Once           // Guards done channel close for idempotent Stop
	config       Config              // Manager configuration
	sender       Sender              // Code delivery implementation
	verification *VerificationSystem // Rate limiting and validation system
}

// NewManager creates a new verification code manager with the provided configuration and sender.
// It initializes the verification system with rate limiting capabilities.
//
// Non-positive CodeLength and CodeExpiresIn values fall back to their defaults,
// matching how NewVerificationSystem treats its own limits. They are normalized
// here rather than at use time because both failure modes are silent: an empty
// code is accepted by Verify from any caller, and an already-expired code makes
// every verification fail while still sending the message.
func NewManager(conf Config, sender Sender) *Manager {
	if conf.CodeLength <= 0 {
		conf.CodeLength = DefaultCodeLength
	}
	if conf.CodeExpiresIn <= 0 {
		conf.CodeExpiresIn = DefaultCodeExpiresIn
	}
	return &Manager{
		done:         make(chan struct{}),
		config:       conf,
		sender:       sender,
		verification: NewVerificationSystem(conf.RateLimit),
	}
}

// SendCode generates and sends a verification code to the specified number.
// It creates a random code, saves it with expiration, and delivers it using the configured sender.
// Returns an error if code generation, storage, or delivery fails.
// Rate-limit rejections are ErrMinuteLimitExceeded and ErrDailyLimitExceeded;
// match them with errors.Is. When delivery fails, the stored code and the
// consumed quota are not rolled back.
func (m *Manager) SendCode(number string) error {
	code := RandomCode(m.config.CodeLength)
	if err := m.verification.SaveCode(number, code, time.Duration(m.config.CodeExpiresIn)*time.Second); err != nil {
		return err
	}
	return m.sender.SendCode(number, code)
}

// Verify validates a verification code for the given number.
// Returns true if the code is valid and not expired, false otherwise.
// A matching code is consumed. See VerificationSystem.Verify for lockout rules.
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
// Start blocks: it returns ctx.Err() when ctx ends, or nil once Stop has been
// called (immediately, if Stop already ran). SendCode and Verify work whether
// or not Start is running.
func (m *Manager) Start(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
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
// Stop is idempotent, does not wait for Start to return, and always returns nil.
func (m *Manager) Stop(ctx context.Context) error {
	m.stopOnce.Do(func() {
		close(m.done)
	})
	return nil
}
