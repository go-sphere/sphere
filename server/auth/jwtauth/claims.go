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

func NewRBACClaims[T authorizer.UID](uid T, subject string, roles []string, expiresAt time.Time) RBACClaims[T] {
	return RBACClaims[T]{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		UID:   uid,
		Roles: roles,
	}
}

// GetUID returns the token's identity, rejecting a zero UID.
//
// The uid claim is emitted with omitempty, so a token minted for another purpose
// with the same signing key simply has no uid and unmarshals to the zero value.
// Returning it with a nil error would authenticate that token as user 0 and pass
// every downstream ownership check, which is why the Claims contract requires an
// error here instead.
func (r RBACClaims[T]) GetUID() (T, error) {
	var zero T
	if r.UID == zero {
		return zero, authorizer.MissingUIDError
	}
	return r.UID, nil
}

func (r RBACClaims[T]) GetRoles() ([]string, error) {
	return r.Roles, nil
}
