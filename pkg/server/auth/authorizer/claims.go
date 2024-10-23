package authorizer

import (
	"errors"
	"golang.org/x/exp/constraints"
	"time"
)

type UID interface {
	constraints.Integer | string
}

var (
	ErrorExpiredToken = errors.New("expired token")
)

const (
	ContextKeyUID     = "uid"
	ContextKeySubject = "subject"
	ContextKeyRoles   = "roles"
)

type RBACClaims[T UID] struct {
	UID       T        `json:"uid,omitempty"`
	Subject   string   `json:"sub,omitempty"`
	Roles     []string `json:"roles,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
}

func NewRBACClaims[T UID](uid T, subject string, roles []string, expiresAt int64) *RBACClaims[T] {
	return &RBACClaims[T]{
		UID:       uid,
		Subject:   subject,
		Roles:     roles,
		ExpiresAt: expiresAt,
	}
}

func (c RBACClaims[T]) Valid() error {
	if c.ExpiresAt < time.Now().Unix() {
		return ErrorExpiredToken
	}
	return nil
}
