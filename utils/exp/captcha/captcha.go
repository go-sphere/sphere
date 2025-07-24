package captcha

import (
	"context"
	"time"
)

type Sender interface {
	SendCode(number string, code string) error
}

type Config struct {
	CodeLength    int                 `json:"code_length"`
	CodeExpiresIn int                 `json:"code_expires_in"`
	RateLimit     *VerificationConfig `json:"rate_limit"`
}

type Manager struct {
	done         chan struct{}
	config       *Config
	sender       Sender
	verification *VerificationSystem
}

func NewManager(config *Config, sender Sender) *Manager {
	return &Manager{
		done:         make(chan struct{}),
		config:       config,
		sender:       sender,
		verification: NewVerificationSystem(config.RateLimit),
	}
}

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

func (m *Manager) Verify(number, code string) bool {
	return m.verification.Verify(number, code)
}

func (m *Manager) Identifier() string {
	return "captcha"
}

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

func (m *Manager) Stop(ctx context.Context) error {
	close(m.done)
	return nil
}
