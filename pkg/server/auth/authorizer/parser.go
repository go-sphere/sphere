package authorizer

import (
	"errors"
	"golang.org/x/exp/constraints"
	"time"
)

var (
	ErrorExpiredToken = errors.New("expired token")
)

type UID interface {
	constraints.Integer | string
}

type Claims[T UID] struct {
	UID       T      `json:"uid,omitempty"`
	Subject   string `json:"sub,omitempty"`
	Roles     string `json:"roles,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
}

func NewClaims[T UID](uid T, subject string, roles string, expiresAt int64) *Claims[T] {
	return &Claims[T]{
		UID:       uid,
		Subject:   subject,
		Roles:     roles,
		ExpiresAt: expiresAt,
	}
}

func (c *Claims[T]) Valid() error {
	if c.ExpiresAt < time.Now().Unix() {
		return ErrorExpiredToken
	}
	return nil
}

type Token struct {
	Token     string
	ExpiresAt time.Time
}

type Parser[T UID] interface {
	ParseToken(token string) (*Claims[T], error)
	ParseRoles(roles string) []string
}

type Generator[T UID] interface {
	GenerateToken(claims *Claims[T]) (*Token, error)
	GenerateRoles(roles []string) string
}

type Authorizer[T UID] interface {
	Parser[T]
	Generator[T]
}
