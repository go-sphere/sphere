package jwtauth

import (
	"time"

	"github.com/go-sphere/sphere/server/auth/authorizer"
	"github.com/golang-jwt/jwt/v5"
)

var _ authorizer.Claims[int64] = (*RBACClaims[int64])(nil)

type RBACClaims[T authorizer.UID] struct {
	jwt.RegisteredClaims
	UID   T        `json:"uid,omitempty"`
	Roles []string `json:"roles,omitempty"`
}

func NewRBACClaims[T authorizer.UID](uid T, subject string, roles []string, expiresAt time.Time) *RBACClaims[T] {
	return &RBACClaims[T]{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		UID:   uid,
		Roles: roles,
	}
}

func (r RBACClaims[T]) GetUID() (T, error) {
	return r.UID, nil
}

func (r RBACClaims[T]) GetRoles() ([]string, error) {
	return r.Roles, nil
}
