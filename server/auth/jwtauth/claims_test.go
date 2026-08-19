package jwtauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/sphere/server/auth/authorizer"
	"github.com/golang-jwt/jwt/v5"
)

// TestGetUIDRejectsZeroValue pins that a token without a uid claim is rejected
// rather than authenticated as the zero user. The claim is serialized with
// omitempty, so any token signed with the same key for a different purpose
// (password reset, email verification, an internal service token) parses
// cleanly and would otherwise pass every "is logged in" check.
func TestGetUIDRejectsZeroValue(t *testing.T) {
	t.Run("int64", func(t *testing.T) {
		var claims RBACClaims[int64]
		if _, err := claims.GetUID(); !errors.Is(err, authorizer.MissingUIDError) {
			t.Fatalf("expected MissingUIDError for zero uid, got %v", err)
		}
		claims.UID = 7
		if uid, err := claims.GetUID(); err != nil || uid != 7 {
			t.Fatalf("expected uid 7, got (%d, %v)", uid, err)
		}
	})

	t.Run("string", func(t *testing.T) {
		var claims RBACClaims[string]
		if _, err := claims.GetUID(); !errors.Is(err, authorizer.MissingUIDError) {
			t.Fatalf("expected MissingUIDError for empty uid, got %v", err)
		}
		claims.UID = "u-1"
		if uid, err := claims.GetUID(); err != nil || uid != "u-1" {
			t.Fatalf("expected uid u-1, got (%q, %v)", uid, err)
		}
	})
}

// TestParsedTokenWithoutUIDIsRejected covers the end-to-end path: a validly
// signed token that simply omits uid must not yield usable auth data.
func TestParsedTokenWithoutUIDIsRejected(t *testing.T) {
	const secret = "shared-signing-key"
	auth := NewJwtAuth[RBACClaims[int64]](secret)

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, RBACClaims[int64]{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "password-reset",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := auth.ParseToken(context.Background(), token)
	if err != nil {
		t.Fatalf("the token itself is valid, ParseToken should succeed: %v", err)
	}
	if _, err := claims.GetUID(); !errors.Is(err, authorizer.MissingUIDError) {
		t.Fatalf("expected MissingUIDError, got %v", err)
	}
}
