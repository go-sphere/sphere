package jwtauth

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/sphere/server/auth/authorizer"
	"github.com/golang-jwt/jwt/v5"
)

func TestJwtAuth_GenerateToken(t *testing.T) {
	claims := NewRBACClaims[int64](12345, "test-subject", []string{"admin", "user"}, time.Now().Add(1*time.Hour))
	jwtAuth := NewJwtAuth[RBACClaims[int64]]("secret")
	token, err := jwtAuth.GenerateToken(context.Background(), claims)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	parsedClaims, err := jwtAuth.ParseToken(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	if parsedClaims.UID != claims.UID {
		t.Errorf("expected UID %d, got %d", claims.UID, parsedClaims.UID)
	}
	if len(parsedClaims.Roles) != len(claims.Roles) {
		t.Errorf("expected roles %v, got %v", claims.Roles, parsedClaims.Roles)
	}
	for i, role := range claims.Roles {
		if parsedClaims.Roles[i] != role {
			t.Errorf("expected role %s, got %s", role, parsedClaims.Roles[i])
		}
	}
	if parsedClaims.Subject != claims.Subject {
		t.Errorf("expected subject %s, got %s", claims.Subject, parsedClaims.Subject)
	}

	jwtAuth2 := NewJwtAuth[RBACClaims[int64]]("secret", WithSigningMethod(jwt.SigningMethodHS512))
	_, err = jwtAuth2.ParseToken(context.Background(), token)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// TestJwtAuth_MalformedTokens tests parsing of adversarial and malformed tokens.
func TestJwtAuth_MalformedTokens(t *testing.T) {
	t.Parallel()

	auth := NewJwtAuth[RBACClaims[int64]]("super-secret")

	// Valid token for reference
	validClaims := NewRBACClaims[int64](999, "alice", []string{"admin"}, time.Now().Add(time.Hour))
	validToken, err := auth.GenerateToken(context.Background(), validClaims)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	malformedTokens := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "single dot", token: "."},
		{name: "two dots", token: ".."},
		{name: "three dots", token: "..."},
		{name: "garbage text", token: "not-a-jwt-token-at-all"},
		{name: "non-base64 header", token: "???." + validToken},
		{name: "non-base64 payload", token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.???." + "sig"},
		{name: "non-json payload", token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." + base64.RawURLEncoding.EncodeToString([]byte("invalid json")) + ".sig"},
		{name: "corrupted signature", token: validToken[:len(validToken)-4] + "AAAA"},
		{name: "truncated token", token: validToken[:len(validToken)/2]},
		{name: "extra segment", token: validToken + ".extra_segment"},
	}

	for _, tc := range malformedTokens {
		t.Run(tc.name, func(t *testing.T) {
			_, err := auth.ParseToken(context.Background(), tc.token)
			if err == nil {
				t.Errorf("ParseToken(%q) succeeded, expected error", tc.token)
			}
		})
	}
}

// TestJwtAuth_AlgorithmMismatchAndNoneAttack tests attack scenarios where
// the signing algorithm is altered or set to none.
func TestJwtAuth_AlgorithmMismatchAndNoneAttack(t *testing.T) {
	t.Parallel()

	secret := "shared-secret"
	authHS256 := NewJwtAuth[RBACClaims[int64]](secret, WithSigningMethod(jwt.SigningMethodHS256))
	authHS512 := NewJwtAuth[RBACClaims[int64]](secret, WithSigningMethod(jwt.SigningMethodHS512))

	// Generate token with HS512
	claims := NewRBACClaims[int64](123, "sub", []string{"user"}, time.Now().Add(time.Hour))
	tokenHS512, err := authHS512.GenerateToken(context.Background(), claims)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	// Verify with HS256 auth -> MUST fail
	_, err = authHS256.ParseToken(context.Background(), tokenHS512)
	if err == nil {
		t.Error("ParseToken with algorithm mismatch must fail, got nil error")
	}

	// Craft 'none' algorithm token
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"uid":123,"sub":"admin","roles":["admin"]}`))
	noneToken := header + "." + payload + "."

	_, err = authHS256.ParseToken(context.Background(), noneToken)
	if err == nil {
		t.Error("ParseToken with 'none' algorithm must fail, got nil error")
	}
}

// TestJwtAuth_TemporalConstraints tests expired tokens and future nbf tokens.
func TestJwtAuth_TemporalConstraints(t *testing.T) {
	t.Parallel()

	auth := NewJwtAuth[RBACClaims[int64]]("secret")

	// 1. Expired token
	expiredClaims := RBACClaims[int64]{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "expired-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-20 * time.Minute)),
		},
		UID: 101,
	}
	expiredToken, err := auth.GenerateToken(context.Background(), expiredClaims)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	_, err = auth.ParseToken(context.Background(), expiredToken)
	if err == nil {
		t.Error("ParseToken on expired token must return error, got nil")
	}

	// 2. Future NotBefore token
	futureClaims := RBACClaims[int64]{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "future-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		UID: 102,
	}
	futureToken, err := auth.GenerateToken(context.Background(), futureClaims)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	_, err = auth.ParseToken(context.Background(), futureToken)
	if err == nil {
		t.Error("ParseToken on future nbf token must return error, got nil")
	}
}

func TestJwtAuth_MissingUIDClaims(t *testing.T) {
	t.Parallel()

	authInt := NewJwtAuth[RBACClaims[int64]]("secret")
	authStr := NewJwtAuth[RBACClaims[string]]("secret")

	// 1. Token with zero int64 UID
	zeroIntClaims := NewRBACClaims[int64](0, "zero-user", []string{"user"}, time.Now().Add(time.Hour))
	tokenZeroInt, err := authInt.GenerateToken(context.Background(), zeroIntClaims)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	parsedInt, err := authInt.ParseToken(context.Background(), tokenZeroInt)
	if err != nil {
		t.Fatalf("ParseToken succeeded: %v", err)
	}
	if _, err := parsedInt.GetUID(); !errors.Is(err, authorizer.MissingUIDError) {
		t.Fatalf("expected MissingUIDError, got %v", err)
	}

	// 2. Token with empty string UID
	zeroStrClaims := NewRBACClaims[string]("", "zero-str-user", []string{"user"}, time.Now().Add(time.Hour))
	tokenZeroStr, err := authStr.GenerateToken(context.Background(), zeroStrClaims)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	parsedStr, err := authStr.ParseToken(context.Background(), tokenZeroStr)
	if err != nil {
		t.Fatalf("ParseToken succeeded: %v", err)
	}
	if _, err := parsedStr.GetUID(); !errors.Is(err, authorizer.MissingUIDError) {
		t.Fatalf("expected MissingUIDError, got %v", err)
	}
}
